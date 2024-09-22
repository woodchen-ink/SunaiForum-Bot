package service

import (
	"fmt"
	"sort"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
)

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
		core.SendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

func handleListKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	keywords, err := core.DB.GetAllKeywords()
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "获取关键词列表时发生错误。")
		logger.Printf("Failed to get keywords: %v", err)
		return
	}

	if len(keywords) == 0 {
		core.SendMessage(bot, message.Chat.ID, "关键词列表为空。")
	} else {
		// 对关键词进行排序
		sort.Strings(keywords)

		// 直接发送排序后的关键词列表
		err := core.SendLongMessage(bot, message.Chat.ID, "当前关键词列表（按字母顺序排序）：", keywords)
		if err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "发送关键词列表时发生错误。")
		}
	}
}

func handleAddKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if keyword == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "请提供要添加的关键词。")
		return
	}

	exists, err := core.DB.KeywordExists(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "检查关键词时发生错误。")
		logger.Printf("Failed to check keyword: %v", err)
		return
	}
	if !exists {
		err = core.DB.AddKeyword(keyword)
		if err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "添加关键词时发生错误。")
			logger.Printf("Failed to add keyword: %v", err)
		} else {
			core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword))
		}
	} else {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword))
	}
}

func handleDeleteKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if keyword == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "请提供要删除的关键词。")
		return
	}

	removed, err := core.DB.RemoveKeyword(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除关键词 '%s' 时发生错误: %v", keyword, err))
		logger.Printf("Failed to remove keyword '%s': %v", keyword, err)
		return
	}

	if removed {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已成功删除。", keyword))
	} else {
		handleSimilarKeywords(bot, message, keyword)
	}
}

func handleSimilarKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	similarKeywords, err := core.DB.SearchKeywords(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "搜索关键词时发生错误。")
		logger.Printf("Failed to search keywords: %v", err)
		return
	}
	if len(similarKeywords) > 0 {
		core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
	} else {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'，且未找到相似的关键词。", keyword))
	}
}

func handleDeleteContainingKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, substring string) {
	if substring == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "请提供要删除的子字符串。")
		return
	}

	removedKeywords, err := core.DB.RemoveKeywordsContaining(substring)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "删除关键词时发生错误。")
		logger.Printf("Failed to remove keywords: %v", err)
		return
	}
	if len(removedKeywords) > 0 {
		core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removedKeywords)
	} else {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring))
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
		core.SendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

func handleListWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	whitelist, err := core.DB.GetAllWhitelist()
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("获取白名单时发生错误: %v", err))
		logger.Printf("Failed to get whitelist: %v", err)
		return
	}
	if len(whitelist) == 0 {
		core.SendMessage(bot, message.Chat.ID, "白名单为空。")
	} else {
		core.SendLongMessage(bot, message.Chat.ID, "白名单域名列表：", whitelist)
	}
}

func handleAddWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if domain == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "请提供要添加的域名。")
		return
	}

	domain = strings.ToLower(domain)
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		logger.Printf("Failed to check whitelist: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain))
		return
	}

	err = core.DB.AddWhitelist(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("添加到白名单时发生错误: %v", err))
		logger.Printf("Failed to add to whitelist: %v", err)
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证添加操作时发生错误: %v", err))
		logger.Printf("Failed to verify add operation: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功添加到白名单。", domain))
	} else {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能添加域名 '%s' 到白名单。", domain))
	}
}

func handleDeleteWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	if domain == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "请提供要删除的域名。")
		return
	}

	domain = strings.ToLower(domain)
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		logger.Printf("Failed to check whitelist: %v", err)
		return
	}
	if !exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain))
		return
	}

	err = core.DB.RemoveWhitelist(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("从白名单删除时发生错误: %v", err))
		logger.Printf("Failed to remove from whitelist: %v", err)
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证删除操作时发生错误: %v", err))
		logger.Printf("Failed to verify delete operation: %v", err)
		return
	}
	if !exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功从白名单中删除。", domain))
	} else {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能从白名单中删除域名 '%s'。", domain))
	}
}
