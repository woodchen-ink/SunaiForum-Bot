package command

// 过滤关键词的维护命令。
// 增删都支持一次多行批量处理 —— 管理员补词通常是一次贴一批, 逐条发命令太累。
import (
	"fmt"
	"log"
	"sort"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// addKeywords 批量添加关键词, 逐条校验, 汇总回报
func addKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	var report batchReport

	for _, keyword := range splitLines(args) {
		if err := core.ValidateKeyword(keyword); err != nil {
			report.failed = append(report.failed, fmt.Sprintf("%s（%v）", keyword, err))
			continue
		}

		added, err := core.DB.AddKeyword(keyword, core.SourceManual)
		if err != nil {
			report.failed = append(report.failed, fmt.Sprintf("%s（写入失败）", keyword))
			log.Printf("[Command] 添加关键词 %q 失败: %v", keyword, err)
			continue
		}
		if added {
			report.succeeded = append(report.succeeded, keyword)
		} else {
			report.skipped = append(report.skipped, keyword)
		}
	}

	core.SendMessage(bot, message.Chat.ID, report.render("已添加", "已存在，跳过"))
}

// deleteKeywords 批量删除关键词; 删掉的若是 AI 加的词, 顺带写入否决表永久拦住它
func deleteKeywords(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	var (
		report   batchReport
		rejected []string
	)

	for _, keyword := range splitLines(args) {
		removed, source, err := core.DB.RemoveKeyword(keyword)
		if err != nil {
			report.failed = append(report.failed, fmt.Sprintf("%s（删除失败）", keyword))
			log.Printf("[Command] 删除关键词 %q 失败: %v", keyword, err)
			continue
		}
		if !removed {
			report.skipped = append(report.skipped, keyword)
			continue
		}

		report.succeeded = append(report.succeeded, keyword)

		// 管理员亲手删掉 AI 加的词 = 否决, AI 不得再添加
		if source == core.SourceAI {
			if err := core.DB.RejectKeyword(keyword); err != nil {
				log.Printf("[Command] 否决关键词 %q 失败: %v", keyword, err)
				continue
			}
			rejected = append(rejected, keyword)
		}
	}

	result := report.render("已删除", "不存在，跳过")
	if len(rejected) > 0 {
		result += fmt.Sprintf("\n\n其中 %s 是 AI 添加的，已永久否决，AI 不会再次添加。", strings.Join(rejected, "、"))
	}
	if len(report.skipped) > 0 {
		result += "\n\n" + suggestSimilar(report.skipped)
	}
	core.SendMessage(bot, message.Chat.ID, result)
}

// suggestSimilar 为没找到的关键词列出相似项, 帮管理员确认库里实际存的是哪一条
func suggestSimilar(missing []string) string {
	var b strings.Builder

	for _, keyword := range missing {
		similar, err := core.DB.SearchKeywords(keyword)
		if err != nil {
			log.Printf("[Command] 搜索相似关键词失败: %v", err)
			continue
		}
		if len(similar) > 0 {
			fmt.Fprintf(&b, "与 %s 相似的有：%s\n", keyword, strings.Join(similar, "、"))
		}
	}

	if b.Len() == 0 {
		return "未找到相似的关键词。"
	}
	return strings.TrimSpace(b.String())
}

// deleteKeywordsContaining 删除所有包含指定子串的关键词
func deleteKeywordsContaining(bot *tgbotapi.BotAPI, message *tgbotapi.Message, substring string) {
	if err := core.ValidateKeyword(substring); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("输入验证失败: %v", err))
		return
	}

	removed, err := core.DB.RemoveKeywordsContaining(substring)
	if err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "删除时发生错误。")
		log.Printf("[Command] 删除包含 %q 的关键词失败: %v", substring, err)
		return
	}

	if len(removed) == 0 {
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring))
		return
	}
	core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removed)
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

	if len(manual) > 0 {
		words := make([]string, 0, len(manual))
		for _, k := range manual {
			words = append(words, k.Word)
		}
		sort.Strings(words)

		if err := core.SendLongMessage(bot, message.Chat.ID,
			fmt.Sprintf("手工关键词（%d 条，按字母排序）：", len(words)), words); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "发送关键词列表时发生错误。")
			return
		}
	}

	if len(aiKeywords) > 0 {
		words := make([]string, 0, len(aiKeywords))
		for _, k := range aiKeywords {
			words = append(words, fmt.Sprintf("%s（命中 %d 次）", k.Word, k.HitCount))
		}
		if err := core.SendLongMessage(bot, message.Chat.ID,
			fmt.Sprintf("AI 关键词（%d 条，按命中次数排序，用 /delete 删除即永久否决）：", len(words)), words); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, "发送 AI 关键词列表时发生错误。")
		}
	}
}
