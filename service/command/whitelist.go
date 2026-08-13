package command

// 管理员的域名白名单维护命令: /addwhite /delwhite /listwhite
import (
	"fmt"
	"log"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleWhitelist 按子命令分发白名单维护操作
func HandleWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd, args string) {
	args = strings.TrimSpace(args)

	switch cmd {
	case "listwhite":
		listWhitelist(bot, message)
	case "addwhite":
		addWhitelist(bot, message, args)
	case "delwhite":
		deleteWhitelist(bot, message, args)
	default:
		core.SendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

func listWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	whitelist, err := core.DB.GetAllWhitelist()
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("获取白名单时发生错误: %v", err))
		log.Printf("[Command] 获取白名单失败: %v", err)
		return
	}

	if len(whitelist) == 0 {
		core.SendMessage(bot, message.Chat.ID, "白名单为空。")
		return
	}
	core.SendLongMessage(bot, message.Chat.ID, "白名单域名列表：", whitelist)
}

func addWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if err := core.ValidateDomain(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}
	domain = strings.TrimSpace(strings.ToLower(domain))

	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		log.Printf("[Command] 检查白名单失败: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain))
		return
	}

	if err := core.DB.AddWhitelist(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("添加到白名单时发生错误: %v", err))
		log.Printf("[Command] 添加白名单失败: %v", err)
		return
	}
	core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功添加到白名单。", domain))
}

func deleteWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if err := core.ValidateDomain(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}
	domain = strings.TrimSpace(strings.ToLower(domain))

	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		log.Printf("[Command] 检查白名单失败: %v", err)
		return
	}
	if !exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain))
		return
	}

	if err := core.DB.RemoveWhitelist(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("从白名单删除时发生错误: %v", err))
		log.Printf("[Command] 删除白名单失败: %v", err)
		return
	}
	core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功从白名单中删除。", domain))
}
