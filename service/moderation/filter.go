package moderation

// 群消息内容审核入口。
//
// 判定 (Inspect) 与处置 (Enforce) 分离: Inspect 是无副作用的纯函数, 可以直接单测;
// Enforce 负责删消息、记分、封禁和通知管理员。
//
// 规则按成本从低到高排列, 命中即返回, 不做多余计算。
import (
	"fmt"
	"log"
	"strings"
	"time"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// logTextLimit 日志与管理员通知中截断原文的字符数
const logTextLimit = 120

// 规则标识, 出现在日志与管理员通知里, 便于回溯是哪条规则判的
const (
	ruleZeroWidth   = "零宽字符"
	ruleObfuscated  = "分隔符拆字"
	ruleKeyword     = "关键词"
	ruleDisplayName = "昵称关键词"
	ruleFlooding    = "重复刷屏"
	ruleAI          = "AI 判定"
)

// Verdict 审核结论
type Verdict struct {
	Hit    bool
	Rule   string // 命中的规则
	Detail string // 命中细节, 如具体关键词或重复次数
}

// Inspect 判定一条消息是否应当拦截, 无副作用。
// displayName 传发送者的昵称与用户名拼接结果; 刷屏计数由调用方通过 repeatCount 传入。
func Inspect(text, displayName string, repeatCount int) Verdict {
	if ContainsZeroWidth(text) {
		return Verdict{Hit: true, Rule: ruleZeroWidth}
	}

	if looksObfuscated(text) {
		return Verdict{Hit: true, Rule: ruleObfuscated}
	}

	keywords, err := core.DB.GetActiveKeywords()
	if err != nil {
		// 查库失败按放行处理: 宁可漏拦, 不可因为数据库抖动误删用户消息
		log.Printf("[Moderation] 读取关键词失败, 本条放行: %v", err)
		return Verdict{}
	}

	if hit := matchKeyword(text, keywords); hit != "" {
		return Verdict{Hit: true, Rule: ruleKeyword, Detail: hit}
	}

	// 昵称带广告词的账号, 其消息一并拦截
	if hit := matchKeyword(displayName, keywords); hit != "" {
		return Verdict{Hit: true, Rule: ruleDisplayName, Detail: hit}
	}

	if repeatCount >= repeatThreshold {
		return Verdict{Hit: true, Rule: ruleFlooding, Detail: fmt.Sprintf("%d 分钟内重复 %d 次", int(repeatWindow.Minutes()), repeatCount)}
	}

	return Verdict{}
}

// matchKeyword 在归一化后的文本里查找命中的关键词, 返回原始关键词形态
func matchKeyword(text string, keywords []string) string {
	normalized := Normalize(text)
	if normalized == "" {
		return ""
	}

	for _, keyword := range keywords {
		normalizedKeyword := Normalize(keyword)
		if normalizedKeyword != "" && strings.Contains(normalized, normalizedKeyword) {
			return keyword
		}
	}
	return ""
}

// CheckAndFilter 审核一条群消息并执行处置, 返回是否已拦截。
// 无论是否命中都会记录内容用于刷屏统计, 因此每条群消息都必须走这里。
func CheckAndFilter(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	text := MessageText(message)
	repeatCount := countRepeat(message.From.ID, text)

	verdict := Inspect(text, DisplayName(message.From), repeatCount)
	if !verdict.Hit {
		return false
	}

	enforce(bot, message, text, verdict, nil)
	return true
}

// countRepeat 记录本条消息并返回相同内容在时间窗内的出现次数; 过短的内容不计
func countRepeat(userID int64, text string) int {
	normalized := Normalize(text)
	if len([]rune(normalized)) < minRepeatLength {
		return 0
	}
	return repeats.record(userID, normalized)
}

// EnforceExternalVerdict 落实由外部判定器 (当前是 AI 审核) 给出的结论。
// learnedWords 是本次判定新增的 AI 关键词, 管理员点撤销时会连同它们一起回滚。
func EnforceExternalVerdict(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text, detail string, learnedWords []string) {
	enforce(bot, message, text, Verdict{Hit: true, Rule: ruleAI, Detail: detail}, learnedWords)
}

// enforce 执行处置: 删消息 -> 记分 -> 达阈值封禁 -> 记录可撤销的处置 -> 通知管理员
func enforce(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string, verdict Verdict, learnedWords []string) {
	user := message.From
	chatID := message.Chat.ID

	log.Printf("[Moderation] 命中「%s」%s, 用户 %d(%s): %s",
		verdict.Rule, verdict.Detail, user.ID, user.UserName, truncate(text, logTextLimit))

	core.DeleteMessages(bot, chatID, message.MessageID)

	// 命中计数用于定期整理: 长期零命中的 AI 词说明提取得不准, 应当清理
	if verdict.Rule == ruleKeyword || verdict.Rule == ruleDisplayName {
		if err := core.DB.RecordKeywordHit(verdict.Detail); err != nil {
			log.Printf("[Moderation] 记录关键词命中失败: %v", err)
		}
	}

	strikes, err := core.DB.AddStrike(user.ID, chatID)
	if err != nil {
		log.Printf("[Moderation] 记录违规次数失败: %v", err)
	}

	banned := false
	if core.AutoBanThreshold > 0 && strikes >= core.AutoBanThreshold {
		if err := core.BanUser(bot, chatID, user.ID); err != nil {
			log.Printf("[Moderation] 自动封禁用户 %d 失败: %v", user.ID, err)
		} else {
			banned = true
			log.Printf("[Moderation] 已自动封禁用户 %d(%s), 累计违规 %d 次", user.ID, user.UserName, strikes)
		}
	}

	// 只在首次违规时在群里留提示, 避免刷屏时机器人跟着刷一遍
	if strikes <= 1 && !banned {
		if sent, err := bot.Send(tgbotapi.NewMessage(chatID, "已撤回该消息。")); err == nil {
			core.DeleteMessageAfterDelay(bot, chatID, sent.MessageID, 3*time.Minute)
		}
	}

	actionID, err := core.DB.RecordModerationAction(core.ModerationAction{
		UserID:       user.ID,
		ChatID:       chatID,
		UserName:     DisplayName(user),
		MessageText:  text,
		Rule:         verdict.Rule,
		LearnedWords: learnedWords,
		Banned:       banned,
	})
	if err != nil {
		log.Printf("[Moderation] 记录处置失败, 本次将无法一键撤销: %v", err)
	}

	notifyAdmin(bot, message, text, verdict, strikes, banned, learnedWords, actionID)
}

// notifyAdmin 把处置结果私聊推给管理员, 附撤销按钮供一键回滚误判
func notifyAdmin(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string,
	verdict Verdict, strikes int, banned bool, learnedWords []string, actionID int64) {

	var b strings.Builder
	b.WriteString("🛡 已撤回一条消息\n\n")
	fmt.Fprintf(&b, "规则: %s", verdict.Rule)
	if verdict.Detail != "" {
		fmt.Fprintf(&b, " (%s)", verdict.Detail)
	}
	fmt.Fprintf(&b, "\n用户: %s (ID: %d)\n", DisplayName(message.From), message.From.ID)
	fmt.Fprintf(&b, "累计违规: %d 次\n", strikes)
	if banned {
		b.WriteString("处置: 已自动封禁并踢出\n")
	}
	if len(learnedWords) > 0 {
		fmt.Fprintf(&b, "新增关键词: %s\n", strings.Join(learnedWords, "、"))
	}
	fmt.Fprintf(&b, "\n原文:\n%s", truncate(text, logTextLimit))

	msg := tgbotapi.NewMessage(core.AdminID, b.String())
	if actionID > 0 {
		msg.ReplyMarkup = undoKeyboard(actionID)
	}
	if _, err := bot.Send(msg); err != nil {
		log.Printf("[Moderation] 通知管理员失败: %v", err)
	}
}

// MessageText 取一条消息中需要审核的全部文本: 正文与图片/视频的说明文字
func MessageText(message *tgbotapi.Message) string {
	if message.Caption != "" {
		if message.Text != "" {
			return message.Text + "\n" + message.Caption
		}
		return message.Caption
	}
	return message.Text
}

// displayName 拼接发送者的昵称与用户名, 用于昵称关键词匹配和通知展示
func DisplayName(user *tgbotapi.User) string {
	parts := make([]string, 0, 3)
	for _, s := range []string{user.FirstName, user.LastName, user.UserName} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// truncate 按字符数截断字符串用于日志输出, 不会切坏多字节字符
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
