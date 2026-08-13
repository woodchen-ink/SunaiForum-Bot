package command

// 管理员的过滤关键词维护命令: /add /delete /list /deletecontaining
import (
	"fmt"
	"log"
	"sort"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// HandleKeyword 按子命令分发关键词维护操作
func HandleKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, cmd, args string) {
	args = strings.TrimSpace(args)

	switch cmd {
	case "list":
		listKeywords(bot, message)
	case "add":
		addKeyword(bot, message, args)
	case "delete":
		deleteKeyword(bot, message, args)
	case "deletecontaining":
		deleteKeywordsContaining(bot, message, args)
	default:
		core.SendErrorMessage(bot, message.Chat.ID, "无效的命令或参数。")
	}
}

// listKeywords 分来源展示词表: 手工词按字母序, AI 词按命中次数降序,
// 让管理员一眼看出哪些 AI 词在真正起作用、哪些是零命中该清理的
func listKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	manual, err := core.DB.GetKeywordsBySource(core.SourceManual)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "获取关键词列表时发生错误。")
		log.Printf("[Command] 获取手工关键词失败: %v", err)
		return
	}
	aiKeywords, err := core.DB.GetKeywordsBySource(core.SourceAI)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "获取 AI 关键词列表时发生错误。")
		log.Printf("[Command] 获取 AI 关键词失败: %v", err)
		return
	}

	if len(manual) == 0 && len(aiKeywords) == 0 {
		core.SendMessage(bot, message.Chat.ID, "关键词列表为空。")
		return
	}

	manualWords := make([]string, 0, len(manual))
	for _, k := range manual {
		manualWords = append(manualWords, k.Word)
	}
	sort.Strings(manualWords)

	if len(manualWords) > 0 {
		if err := core.SendLongMessage(bot, message.Chat.ID,
			fmt.Sprintf("手工关键词（%d 条，按字母排序）：", len(manualWords)), manualWords); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "发送关键词列表时发生错误。")
			return
		}
	}

	if len(aiKeywords) > 0 {
		aiWords := make([]string, 0, len(aiKeywords))
		for _, k := range aiKeywords {
			aiWords = append(aiWords, fmt.Sprintf("%s（命中 %d 次）", k.Word, k.HitCount))
		}
		if err := core.SendLongMessage(bot, message.Chat.ID,
			fmt.Sprintf("AI 关键词（%d 条，按命中次数排序，删除即永久否决）：", len(aiWords)), aiWords); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "发送 AI 关键词列表时发生错误。")
		}
	}
}

func addKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if err := core.ValidateKeyword(keyword); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}
	keyword = strings.TrimSpace(keyword)

	exists, err := core.DB.KeywordExists(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "检查关键词时发生错误。")
		log.Printf("[Command] 检查关键词失败: %v", err)
		return
	}
	if exists {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword))
		return
	}

	if _, err := core.DB.AddKeyword(keyword, core.SourceManual); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "添加关键词时发生错误。")
		log.Printf("[Command] 添加关键词失败: %v", err)
		return
	}
	core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword))
}

func deleteKeyword(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	if err := core.ValidateKeyword(keyword); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}
	keyword = strings.TrimSpace(keyword)

	removed, err := core.DB.RemoveKeyword(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除关键词 '%s' 时发生错误: %v", keyword, err))
		log.Printf("[Command] 删除关键词 '%s' 失败: %v", keyword, err)
		return
	}

	if removed {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("关键词 '%s' 已成功删除。", keyword))
		return
	}
	suggestSimilarKeywords(bot, message, keyword)
}

// suggestSimilarKeywords 删除未命中时列出相似关键词, 方便管理员确认实际存的是哪一条
func suggestSimilarKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message, keyword string) {
	similar, err := core.DB.SearchKeywords(keyword)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "搜索关键词时发生错误。")
		log.Printf("[Command] 搜索关键词失败: %v", err)
		return
	}

	if len(similar) == 0 {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'，且未找到相似的关键词。", keyword))
		return
	}
	core.SendLongMessage(bot, message.Chat.ID,
		fmt.Sprintf("未能删除关键词 '%s'。\n\n以下是相似的关键词：", keyword), similar)
}

func deleteKeywordsContaining(bot *tgbotapi.BotAPI, message *tgbotapi.Message, substring string) {
	if err := core.ValidateKeyword(substring); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}
	substring = strings.TrimSpace(substring)

	removed, err := core.DB.RemoveKeywordsContaining(substring)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除包含 '%s' 的关键词时发生错误: %v", substring, err))
		log.Printf("[Command] 删除包含 '%s' 的关键词失败: %v", substring, err)
		return
	}

	if len(removed) == 0 {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring))
		return
	}
	core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removed)
}
