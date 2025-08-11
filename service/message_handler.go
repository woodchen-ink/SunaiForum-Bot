// 消息处理函数
package service

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/SunaiForum-Bot/core"
	"github.com/woodchen-ink/SunaiForum-Bot/service/binance"
	"github.com/woodchen-ink/SunaiForum-Bot/service/group_member_management"
	"github.com/woodchen-ink/SunaiForum-Bot/service/link_filter"
	"github.com/woodchen-ink/SunaiForum-Bot/service/prompt_reply"
)

var (
	logger = log.New(log.Writer(), "MessageHandler: ", log.Ldate|log.Ltime|log.Lshortfile)
)

// handleUpdate 处理所有传入的更新信息，包括消息和命令, 然后分开处理。
func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, linkFilter *link_filter.LinkFilter, rateLimiter *core.RateLimiter) {
	// 检查更新是否包含消息，如果不包含则直接返回。
	if update.Message == nil {
		return
	}

	// 如果消息来自私聊且发送者是预定义的管理员，调用处理管理员命令的函数。
	if update.Message.Chat.Type == "private" && update.Message.From.ID == core.ADMIN_ID {
		handleAdminCommand(bot, update.Message)
		return
	}

	// 如果消息来自群聊且通过了速率限制器的检查，调用处理普通消息的函数。
	if update.Message.Chat.Type != "private" && rateLimiter.Allow() {
		processMessage(bot, update.Message, linkFilter)
	}
}

// 处理管理员私聊消息
func handleAdminCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "add", "delete", "list", "deletecontaining":
		HandleKeywordCommand(bot, message, command, args)
	case "addwhite", "delwhite", "listwhite":
		HandleWhitelistCommand(bot, message, command, args)
	case "prompt":
		prompt_reply.HandlePromptCommand(bot, message)
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "未知命令, 听不懂"))
	}
}

// processMessage 处理群里接收到的消息。
func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, linkFilter *link_filter.LinkFilter) {
	// 记录消息内容
	// log.Printf("Processing message: %s", message.Text)
	logger.Printf("Processing message: %s", message.Text)

	// 处理 /ban 命令
	if message.ReplyToMessage != nil && message.Text == "/ban" {
		group_member_management.HandleBanCommand(bot, message)
		return
	}

	// 如果不是管理员，才进行链接过滤
	if !core.IsAdmin(message.From.ID) {
		// 使用新的 CheckAndFilterLink 函数
		if linkFilter.CheckAndFilterLink(bot, message) {
			return
		}
	}

	// 调用 HandleSymbolQuery 处理虚拟币名查询
	binance.HandleSymbolQuery(bot, message)

	// 调用 CheckAndReplyPrompt 函数进行提示词回复
	prompt_reply.CheckAndReplyPrompt(bot, message)
}

func RunMessageHandler() error {
	logger.Println("消息处理器启动...")

	// 加载提示回复数据
	err := prompt_reply.Manager.LoadDataFromDatabase()
	if err != nil {
		logger.Printf("加载提示回复数据失败: %v", err)
		// 考虑是否要因为这个错误停止启动
		// return fmt.Errorf("加载提示回复数据失败: %w", err)
	}

	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	delay := baseDelay

	for {
		err := func() error {
			logger.Printf("Attempting to create bot with token: %s", core.BOT_TOKEN)
			bot, err := tgbotapi.NewBotAPI(core.BOT_TOKEN)
			if err != nil {
				log.Printf("Error details: %+v", err)
				return fmt.Errorf("failed to create bot: %w", err)
			}

			bot.Debug = core.DEBUG_MODE

			logger.Printf("Authorized on account %s", bot.Self.UserName)

			err = core.RegisterCommands(bot)
			if err != nil {
				return fmt.Errorf("error registering commands: %w", err)
			}

			linkFilter, err := link_filter.NewLinkFilter()
			if err != nil {
				log.Printf("Failed to create LinkFilter: %v", err)
				return fmt.Errorf("failed to create LinkFilter: %v", err)
			}

			rateLimiter := core.NewRateLimiter()

			u := tgbotapi.NewUpdate(0)
			u.Timeout = 60

			updates := bot.GetUpdatesChan(u)

			for update := range updates {
				go handleUpdate(bot, update, linkFilter, rateLimiter)
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
			logger.Println("Bot disconnected. Attempting to restart immediately...")
		}
	}
}
