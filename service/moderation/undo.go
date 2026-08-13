package moderation

// 处置撤销: 管理员在通知消息上点一下按钮, 即可完整回滚一次误判。
//
// 撤销是整套自动化的安全阀, 也是 AI 的负反馈信号 —— 它同时做四件事:
//  1. 解封用户 (若已被自动封禁)
//  2. 扣回本次违规计分
//  3. 删除本次 AI 学到的关键词, 并写入否决表, AI 不得再添加
//  4. 把被删的原文重新发回群里
//
// 第 3 步是关键: 误判不只是撤销一次动作, 还要阻止同样的误判再次发生。
import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// undoCallbackPrefix 撤销按钮的 callback_data 前缀。
// callback_data 上限 64 字节, 因此只放处置 id, 详情回表查。
const undoCallbackPrefix = "mdundo:"

// undoKeyboard 构造撤销按钮
func undoKeyboard(actionID int64) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("↩️ 误判，恢复", undoCallbackPrefix+strconv.FormatInt(actionID, 10)),
		),
	)
}

// IsUndoCallback 判断回调是否属于本模块
func IsUndoCallback(data string) bool {
	return strings.HasPrefix(data, undoCallbackPrefix)
}

// HandleUndoCallback 处理撤销按钮点击。仅管理员可用。
func HandleUndoCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	if query.From == nil || !core.IsAdmin(query.From.ID) {
		answerCallback(bot, query.ID, "只有管理员可以操作")
		return
	}

	actionID, err := strconv.ParseInt(strings.TrimPrefix(query.Data, undoCallbackPrefix), 10, 64)
	if err != nil {
		answerCallback(bot, query.ID, "无效的操作")
		return
	}

	action, err := core.DB.GetModerationAction(actionID)
	if err != nil {
		log.Printf("[Moderation] 读取处置记录 %d 失败: %v", actionID, err)
		answerCallback(bot, query.ID, "找不到这条处置记录")
		return
	}

	// 幂等: 重复点击不应该反复解封、反复扣分
	claimed, err := core.DB.MarkActionUndone(actionID)
	if err != nil {
		log.Printf("[Moderation] 标记撤销失败: %v", err)
		answerCallback(bot, query.ID, "操作失败")
		return
	}
	if !claimed {
		answerCallback(bot, query.ID, "这条已经恢复过了")
		return
	}

	summary := undoAction(bot, action)
	answerCallback(bot, query.ID, "已恢复")
	markNotificationUndone(bot, query, summary)
	log.Printf("[Moderation] 管理员撤销了处置 %d (用户 %d): %s", actionID, action.UserID, summary)
}

// undoAction 执行实际的回滚动作, 返回给管理员看的结果摘要。
// 每一步失败都只记录不中断 —— 撤销是补救动作, 能回滚多少是多少。
func undoAction(bot *tgbotapi.BotAPI, action core.ModerationAction) string {
	var done []string

	if action.Banned {
		unbanConfig := tgbotapi.UnbanChatMemberConfig{
			ChatMemberConfig: tgbotapi.ChatMemberConfig{ChatID: action.ChatID, UserID: action.UserID},
			OnlyIfBanned:     true,
		}
		if _, err := bot.Request(unbanConfig); err != nil {
			log.Printf("[Moderation] 解封用户 %d 失败: %v", action.UserID, err)
		} else {
			done = append(done, "已解封")
		}
	}

	if err := core.DB.DecrementStrike(action.UserID, action.ChatID); err != nil {
		log.Printf("[Moderation] 扣回违规计分失败: %v", err)
	} else {
		done = append(done, "已扣回 1 次计分")
	}

	// 撤销属于"否决"语义: 删词之外还要显式拉黑, 阻止 AI 下次再提取同一个词
	var rejected []string
	for _, word := range action.LearnedWords {
		removed, _, err := core.DB.RemoveKeyword(word)
		if err != nil {
			log.Printf("[Moderation] 删除误加关键词 %q 失败: %v", word, err)
			continue
		}
		if !removed {
			continue
		}
		if err := core.DB.RejectKeyword(word); err != nil {
			log.Printf("[Moderation] 否决关键词 %q 失败: %v", word, err)
		}
		rejected = append(rejected, word)
	}
	if len(rejected) > 0 {
		done = append(done, fmt.Sprintf("已删除并永久否决关键词: %s", strings.Join(rejected, "、")))
	}

	if restored := restoreMessage(bot, action); restored {
		done = append(done, "已把原消息发回群里")
	}

	if len(done) == 0 {
		return "没有需要回滚的动作"
	}
	return strings.Join(done, "；")
}

// restoreMessage 把被删的原文重新发回群里。
// Telegram 无法真正恢复已删除的消息, 只能由机器人代为转述并注明原作者。
func restoreMessage(bot *tgbotapi.BotAPI, action core.ModerationAction) bool {
	if strings.TrimSpace(action.MessageText) == "" {
		return false
	}

	text := fmt.Sprintf("↩️ 以下消息为误判撤回，现已恢复\n来自 %s：\n\n%s", action.UserName, action.MessageText)
	if _, err := bot.Send(tgbotapi.NewMessage(action.ChatID, text)); err != nil {
		log.Printf("[Moderation] 恢复原消息失败: %v", err)
		return false
	}
	return true
}

// markNotificationUndone 改写管理员那条通知, 去掉按钮并附上回滚结果
func markNotificationUndone(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery, summary string) {
	if query.Message == nil {
		return
	}

	newText := query.Message.Text + "\n\n✅ 已由管理员恢复\n" + summary
	edit := tgbotapi.NewEditMessageText(query.Message.Chat.ID, query.Message.MessageID, newText)
	if _, err := bot.Request(edit); err != nil {
		log.Printf("[Moderation] 更新通知消息失败: %v", err)
	}
}

// answerCallback 回应按钮点击, 让客户端的加载动画停下来
func answerCallback(bot *tgbotapi.BotAPI, queryID, text string) {
	if _, err := bot.Request(tgbotapi.NewCallback(queryID, text)); err != nil {
		log.Printf("[Moderation] 回应回调失败: %v", err)
	}
}
