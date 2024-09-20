// 消息处理函数
package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
	"github.com/woodchen-ink/Q58Bot/service/group_member_management"
	"github.com/woodchen-ink/Q58Bot/service/link_filter"
	"github.com/woodchen-ink/Q58Bot/service/prompt_reply"
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
	log.Printf("Processing message: %s", message.Text)

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

	// 使用现有的 CheckAndReplyPrompt 函数进行提示词回复
	prompt_reply.CheckAndReplyPrompt(bot, message)
}

func RunMessageHandler() error {
	log.Println("消息处理器启动...")

	// 加载提示回复数据
	err := prompt_reply.Manager.LoadDataFromDatabase()
	if err != nil {
		log.Printf("加载提示回复数据失败: %v", err)
		// 考虑是否要因为这个错误停止启动
		// return fmt.Errorf("加载提示回复数据失败: %w", err)
	}

	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	delay := baseDelay

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
			log.Println("Bot disconnected. Attempting to restart immediately...")
		}
	}
}

// 下面是辅助函数部分
//
//
//

const (
	maxMessageLength = 4000
)

func SendLongMessage(bot *tgbotapi.BotAPI, chatID int64, prefix string, items []string) error {
	message := prefix + "\n"
	for i, item := range items {
		newLine := fmt.Sprintf("%d. %s\n", i+1, item)
		if len(message)+len(newLine) > maxMessageLength {
			if err := sendMessage(bot, chatID, message); err != nil {
				return err
			}
			message = ""
		}
		message += newLine
	}

	if message != "" {
		return sendMessage(bot, chatID, message)
	}

	return nil
}

func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	return err
}

func sendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, errMsg string) {
	sendMessage(bot, chatID, errMsg)
}

func HandleKeywordCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string) {
	args = strings.TrimSpace(args)

	switch command {
	case "list":
		handleListKeywords(bot, message)
	case "add":
		handleAddKeyword(bot, message, args)
	case "delete":
		handleDeleteKeyword(bot, message, args)
	case "deletecontaining":
		handleDeleteContainingKeyword(bot, message, args)
	default:
		sendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

func handleListKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	keywords, err := core.DB.GetAllKeywords()
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, "获取关键词列表时发生错误。")
		return
	}
	if len(keywords) == 0 {
		sendMessage(bot, message.Chat.ID, "关键词列表为空。")
	} else {
		SendLongMessage(bot, message.Chat.ID, "当前关键词列表：", keywords)
	}
}

func handleAddKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if keyword == "" {
		sendErrorMessage(bot, message.Chat.ID, "请提供要添加的关键词。")
		return
	}

	exists, err := core.DB.KeywordExists(keyword)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, "检查关键词时发生错误。")
		return
	}
	if !exists {
		err = core.DB.AddKeyword(keyword)
		if err != nil {
			sendErrorMessage(bot, message.Chat.ID, "添加关键词时发生错误。")
		} else {
			sendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword))
		}
	} else {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword))
	}
}

func handleDeleteKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if keyword == "" {
		sendErrorMessage(bot, message.Chat.ID, "请提供要删除的关键词。")
		return
	}

	removed, err := core.DB.RemoveKeyword(keyword)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除关键词 '%s' 时发生错误: %v", keyword, err))
		return
	}

	if removed {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已成功删除。", keyword))
	} else {
		handleSimilarKeywords(bot, message, keyword)
	}
}

func handleSimilarKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	similarKeywords, err := core.DB.SearchKeywords(keyword)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, "搜索关键词时发生错误。")
		return
	}
	if len(similarKeywords) > 0 {
		SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
	} else {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'，且未找到相似的关键词。", keyword))
	}
}

func handleDeleteContainingKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, substring string) {
	if substring == "" {
		sendErrorMessage(bot, message.Chat.ID, "请提供要删除的子字符串。")
		return
	}

	removedKeywords, err := core.DB.RemoveKeywordsContaining(substring)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, "删除关键词时发生错误。")
		return
	}
	if len(removedKeywords) > 0 {
		SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removedKeywords)
	} else {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring))
	}
}

func HandleWhitelistCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string) {
	args = strings.TrimSpace(args)

	switch command {
	case "listwhite":
		handleListWhitelist(bot, message)
	case "addwhite":
		handleAddWhitelist(bot, message, args)
	case "delwhite":
		handleDeleteWhitelist(bot, message, args)
	default:
		sendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

func handleListWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	whitelist, err := core.DB.GetAllWhitelist()
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("获取白名单时发生错误: %v", err))
		return
	}
	if len(whitelist) == 0 {
		sendMessage(bot, message.Chat.ID, "白名单为空。")
	} else {
		SendLongMessage(bot, message.Chat.ID, "白名单域名列表：", whitelist)
	}
}

func handleAddWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if domain == "" {
		sendErrorMessage(bot, message.Chat.ID, "请提供要添加的域名。")
		return
	}

	domain = strings.ToLower(domain)
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		return
	}
	if exists {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain))
		return
	}

	err = core.DB.AddWhitelist(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("添加到白名单时发生错误: %v", err))
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证添加操作时发生错误: %v", err))
		return
	}
	if exists {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功添加到白名单。", domain))
	} else {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能添加域名 '%s' 到白名单。", domain))
	}
}

func handleDeleteWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if domain == "" {
		sendErrorMessage(bot, message.Chat.ID, "请提供要删除的域名。")
		return
	}

	domain = strings.ToLower(domain)
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		return
	}
	if !exists {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain))
		return
	}

	err = core.DB.RemoveWhitelist(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("从白名单删除时发生错误: %v", err))
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证删除操作时发生错误: %v", err))
		return
	}
	if !exists {
		sendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功从白名单中删除。", domain))
	} else {
		sendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能从白名单中删除域名 '%s'。", domain))
	}
}

// ShouldFilter 检查消息是否包含关键词或者非白名单链接
