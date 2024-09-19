// 消息处理函数
package service

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
	"github.com/woodchen-ink/Q58Bot/service/group_member_management"
	"github.com/woodchen-ink/Q58Bot/service/link_filter"
	"github.com/woodchen-ink/Q58Bot/service/prompt_reply"
)

// handleUpdate 处理所有传入的更新信息，包括消息和命令, 然后分开处理。
func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, linkFilter *link_filter.LinkFilter, rateLimiter *core.RateLimiter, db *core.Database) {
	// 检查更新是否包含消息，如果不包含则直接返回。
	if update.Message == nil {
		return
	}

	// 如果消息来自私聊且发送者是预定义的管理员，调用处理管理员命令的函数。
	if update.Message.Chat.Type == "private" && update.Message.From.ID == core.ADMIN_ID {
		handleAdminCommand(bot, update.Message, db)
		return
	}

	// 如果消息来自群聊且通过了速率限制器的检查，调用处理普通消息的函数。
	if update.Message.Chat.Type != "private" && rateLimiter.Allow() {
		processMessage(bot, update.Message, linkFilter)
	}
}

// 处理管理员私聊消息
func handleAdminCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, db *core.Database) {
	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "add", "delete", "list", "deletecontaining":
		HandleKeywordCommand(bot, message, command, args, db)
	case "addwhite", "delwhite", "listwhite":
		HandleWhitelistCommand(bot, message, command, args, db)
	case "prompt":
		prompt_reply.HandlePromptCommand(bot, message)
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "未知命令, 听不懂"))
	}
}

// processMessage 处理群里接收到的消息。
func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, linkFilter *link_filter.LinkFilter) {
	// 记录消息内容
	log.Printf("Processing message: %s", message.Text)

	// 处理 /ban 命令
	if message.ReplyToMessage != nil && message.Text == "/ban" {
		group_member_management.HandleBanCommand(bot, message)
		return
	}

	// 如果不是管理员，才进行链接过滤
	if !core.IsAdmin(message.From.ID) {
		// 判断消息是否应当被过滤及找出新的非白名单链接
		shouldFilter, newLinks := linkFilter.ShouldFilter(message.Text)
		if shouldFilter {
			// 记录被过滤的消息
			log.Printf("消息应该被过滤: %s", message.Text)
			// 删除原始消息
			deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
			_, err := bot.Request(deleteMsg)
			if err != nil {
				// 删除消息失败时记录错误
				log.Printf("删除消息失败: %v", err)
			}

			// 发送提示消息
			notification := tgbotapi.NewMessage(message.Chat.ID, "已撤回该消息。注:一个链接不能发两次.")
			sent, err := bot.Send(notification)
			if err != nil {
				// 发送通知失败时记录错误
				log.Printf("发送通知失败: %v", err)
			} else {
				// 3分钟后删除提示消息
				go deleteMessageAfterDelay(bot, message.Chat.ID, sent.MessageID, 3*time.Minute)
			}
			// 结束处理
			return
		}
		// 如果发现新的非白名单链接
		if len(newLinks) > 0 {
			// 记录新的非白名单链接
			log.Printf("发现新的非白名单链接: %v", newLinks)
		}
	}

	// 检查消息文本是否匹配预设的提示词并回复
	if reply, found := prompt_reply.GetPromptReply(message.Text); found {
		// 创建回复消息
		replyMsg := tgbotapi.NewMessage(message.Chat.ID, reply)
		replyMsg.ReplyToMessageID = message.MessageID
		sent, err := bot.Send(replyMsg)
		if err != nil {
			// 发送回复失败时记录错误
			log.Printf("未能发送及时回复: %v", err)
		} else {
			// 3分钟后删除回复消息
			go deleteMessageAfterDelay(bot, message.Chat.ID, sent.MessageID, 3*time.Minute)
		}
	}
}

func RunMessageHandler() error {
	log.Println("消息处理器启动...")

	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	delay := baseDelay
	db, err := core.NewDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer db.Close() // 确保在函数结束时关闭数据库连接

	for {
		err := func() error {
			log.Printf("Attempting to create bot with token: %s", core.BOT_TOKEN)
			bot, err := tgbotapi.NewBotAPI(core.BOT_TOKEN)
			if err != nil {
				log.Printf("Error details: %+v", err)
				return fmt.Errorf("failed to create bot: %w", err)
			}

			bot.Debug = core.DEBUG_MODE

			log.Printf("Authorized on account %s", bot.Self.UserName)

			err = core.RegisterCommands(bot)
			if err != nil {
				return fmt.Errorf("error registering commands: %w", err)
			}

			linkFilter, err := link_filter.NewLinkFilter()
			if err != nil {
				return fmt.Errorf("failed to create LinkFilter: %v", err)
			}

			rateLimiter := core.NewRateLimiter()

			u := tgbotapi.NewUpdate(0)
			u.Timeout = 60

			updates := bot.GetUpdatesChan(u)

			for update := range updates {
				go handleUpdate(bot, update, linkFilter, rateLimiter, db)
			}

			return nil
		}()

		if err != nil {
			log.Printf("Bot encountered an error: %v", err)
			log.Printf("Attempting to restart in %v...", delay)
			time.Sleep(delay)

			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			delay = baseDelay
			log.Println("Bot disconnected. Attempting to restart immediately...")
		}
	}
}
