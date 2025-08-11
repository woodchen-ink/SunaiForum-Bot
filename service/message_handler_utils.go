package service

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
	manualKeywords, err := core.DB.GetAllManualKeywords()
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "获取手动添加的关键词列表时发生错误。")
		log.Printf("[MessageHandler] Failed to get manual keywords: %v", err)
		return
	}

	autoAddedLinks, err := core.DB.GetAllAutoAddedLinks()
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "获取自动添加的链接列表时发生错误。")
		log.Printf("[MessageHandler] Failed to get auto-added links: %v", err)
		return
	}

	if len(manualKeywords) == 0 && len(autoAddedLinks) == 0 {
		core.SendMessage(bot, message.Chat.ID, "关键词和链接列表为空。")
	} else {
		// 对关键词和链接进行排序
		sort.Strings(manualKeywords)
		sort.Strings(autoAddedLinks)

		// 发送手动添加的关键词列表
		if len(manualKeywords) > 0 {
			err := core.SendLongMessage(bot, message.Chat.ID, "手动添加的关键词列表（按字母顺序排序）：", manualKeywords)
			if err != nil {
				core.SendErrorMessage(bot, message.Chat.ID, "发送手动添加的关键词列表时发生错误。")
			}
		}

		// 发送自动添加的链接列表
		if len(autoAddedLinks) > 0 {
			err := core.SendLongMessage(bot, message.Chat.ID, "自动添加的链接列表（按字母顺序排序）：", autoAddedLinks)
			if err != nil {
				core.SendErrorMessage(bot, message.Chat.ID, "发送自动添加的链接列表时发生错误。")
			}
		}
	}
}

func handleAddKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	// 输入验证
	if err := core.ValidateKeyword(keyword); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	keyword = strings.TrimSpace(keyword)
	exists, err := core.DB.KeywordExists(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "检查关键词时发生错误。")
		log.Printf("[MessageHandler] Failed to check keyword: %v", err)
		return
	}
	if !exists {
		err = core.DB.AddKeyword(keyword, false, false) // isLink = false, isAutoAdded = false
		if err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "添加关键词时发生错误。")
			log.Printf("[MessageHandler] Failed to add keyword: %v", err)
		} else {
			core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword))
		}
	} else {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword))
	}
}

func handleDeleteKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	// 输入验证
	if err := core.ValidateKeyword(keyword); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	keyword = strings.TrimSpace(keyword)
	removed, err := core.DB.RemoveKeyword(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除关键词 '%s' 时发生错误: %v", keyword, err))
		log.Printf("[MessageHandler] Failed to remove keyword '%s': %v", keyword, err)
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
		log.Printf("[MessageHandler] 搜索关键词时发生错误: %v", err)
		return
	}
	if len(similarKeywords) > 0 {
		core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
	} else {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'，且未找到相似的关键词。", keyword))
	}
}

func handleDeleteContainingKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, substring string) {
	// 输入验证
	if err := core.ValidateKeyword(substring); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	substring = strings.TrimSpace(substring)
	removedKeywords, err := core.DB.RemoveKeywordsContaining(substring)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除包含 '%s' 的关键词时发生错误: %v", substring, err))
		log.Printf("[MessageHandler] 删除包含 '%s' 的关键词时发生错误: %v", substring, err)
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
		log.Printf("[MessageHandler] 获取白名单时发生错误: %v", err)
		return
	}
	if len(whitelist) == 0 {
		core.SendMessage(bot, message.Chat.ID, "白名单为空。")
	} else {
		core.SendLongMessage(bot, message.Chat.ID, "白名单域名列表：", whitelist)
	}
}

func handleAddWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	// 输入验证
	if err := core.ValidateDomain(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	domain = strings.TrimSpace(strings.ToLower(domain))
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		log.Printf("[MessageHandler] 检查白名单时发生错误: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain))
		return
	}

	err = core.DB.AddWhitelist(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("添加到白名单时发生错误: %v", err))
		log.Printf("[MessageHandler] 添加到白名单时发生错误: %v", err)
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证添加操作时发生错误: %v", err))
		log.Printf("[MessageHandler] 验证添加操作时发生错误: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功添加到白名单。", domain))
	} else {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能添加域名 '%s' 到白名单。", domain))
	}
}

func handleDeleteWhitelist(bot *tgbotapi.BotAPI, message *tgbotapi.Message, domain string) {
	// 输入验证
	if err := core.ValidateDomain(domain); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	domain = strings.TrimSpace(strings.ToLower(domain))
	exists, err := core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err))
		log.Printf("[MessageHandler] 检查白名单时发生错误: %v", err)
		return
	}
	if !exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain))
		return
	}

	err = core.DB.RemoveWhitelist(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("从白名单删除时发生错误: %v", err))
		log.Printf("[MessageHandler] 从白名单删除时发生错误: %v", err)
		return
	}

	exists, err = core.DB.WhitelistExists(domain)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("验证删除操作时发生错误: %v", err))
		log.Printf("[MessageHandler] 验证删除操作时发生错误: %v", err)
		return
	}
	if !exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功从白名单中删除。", domain))
	} else {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("未能从白名单中删除域名 '%s'。", domain))
	}
}
