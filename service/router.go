package service

// 消息路由: 按来源 (管理员私聊 / 群聊) 把更新分发到对应处理器
import (
	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/ai_review"
	"SunaiForum-Bot/service/binance"
	"SunaiForum-Bot/service/command"
	"SunaiForum-Bot/service/group_member_management"
	"SunaiForum-Bot/service/moderation"
	"SunaiForum-Bot/service/prompt_reply"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleUpdate 分流一条更新。
// 编辑后的消息同样要过审核 —— 先发正常内容再编辑成广告是常见的规避手法。
func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, rateLimiter *core.RateLimiter) {
	// 管理员在处置通知上点"恢复"按钮
	if query := update.CallbackQuery; query != nil {
		if moderation.IsUndoCallback(query.Data) {
			moderation.HandleUndoCallback(bot, query)
		}
		return
	}

	if edited := update.EditedMessage; edited != nil {
		if edited.From != nil && edited.Chat.Type != "private" && !core.IsAdmin(edited.From.ID) {
			moderation.CheckAndFilter(bot, edited)
		}
		return
	}

	// From 为 nil 的情况确实存在 (匿名管理员、频道身份发言), 不能直接取 ID
	if update.Message == nil || update.Message.From == nil {
		return
	}
	message := update.Message

	if message.Chat.Type == "private" {
		if core.IsAdmin(message.From.ID) {
			handleAdminCommand(bot, message)
		}
		return
	}

	processMessage(bot, message, rateLimiter)
}

// handleAdminCommand 处理管理员私聊里的管理命令
func handleAdminCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	cmd := message.Command()
	args := message.CommandArguments()

	switch cmd {
	case "add", "delete", "list", "deletecontaining":
		command.HandleKeyword(bot, message, cmd, args)
	case "addwhite", "delwhite", "listwhite":
		command.HandleWhitelist(bot, message, cmd, args)
	case "prompt":
		prompt_reply.HandlePromptCommand(bot, message)
	default:
		core.SendErrorMessage(bot, message.Chat.ID, "未知命令, 听不懂")
	}
}

// processMessage 处理群消息。
//
// 内容审核**不受限流约束**: 限流的本意是防止机器人被消息洪水拖垮, 但如果连审核都跳过,
// 刷屏时反而是广告全部漏过。因此限流只作用于机器人的主动响应 (行情查询、自动回复)。
func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, rateLimiter *core.RateLimiter) {
	// 群务通知 (加入/退出/改群名) 没有正文, 清理掉即可, 不必走后续任何处理
	if group_member_management.CleanServiceMessage(bot, message) {
		return
	}

	if message.ReplyToMessage != nil && message.Command() == "ban" {
		group_member_management.HandleBanCommand(bot, message)
		return
	}

	if !core.IsAdmin(message.From.ID) {
		// 确定性规则先跑, 命中即拦截, 不产生 AI 调用
		if moderation.CheckAndFilter(bot, message) {
			return
		}
		// 未命中的交给 AI 复核; 内部自行判断是否值得调用, 且异步执行不阻塞本函数
		ai_review.MaybeReview(bot, message)
	}

	if !rateLimiter.Allow() {
		return
	}

	binance.HandleSymbolQuery(bot, message)
	prompt_reply.CheckAndReplyPrompt(bot, message)
}
