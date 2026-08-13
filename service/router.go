package service

// 消息路由: 按来源 (管理员私聊 / 群聊) 把更新分发到对应处理器
import (
	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/binance"
	"SunaiForum-Bot/service/command"
	"SunaiForum-Bot/service/group_member_management"
	"SunaiForum-Bot/service/moderation"
	"SunaiForum-Bot/service/prompt_reply"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// handleUpdate 分流一条更新: 管理员私聊走命令处理, 群消息走内容审核与功能响应
func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, rateLimiter *core.RateLimiter) {
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

	if rateLimiter.Allow() {
		processMessage(bot, message)
	}
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

// processMessage 处理群消息: 先审核内容, 未被拦截的再交给各功能模块
func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.ReplyToMessage != nil && message.Command() == "ban" {
		group_member_management.HandleBanCommand(bot, message)
		return
	}

	// 管理员不受内容过滤限制
	if !core.IsAdmin(message.From.ID) && moderation.CheckAndFilter(bot, message) {
		return
	}

	binance.HandleSymbolQuery(bot, message)
	prompt_reply.CheckAndReplyPrompt(bot, message)
}
