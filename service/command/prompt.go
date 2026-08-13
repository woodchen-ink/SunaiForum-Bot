package command

// 自动回复的维护命令。
//
// 原先是 /prompt set <词> <回复> 这种子命令形式, 从 Telegram 菜单点击时只能发出裸 /prompt,
// 参数带不了, 必然报错。拆成三个独立命令后每个都能从菜单直接用。
import (
	"fmt"
	"log"
	"strings"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/prompt_reply"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// setPrompt 设置一条自动回复: 第一行是触发词, 之后所有行是回复内容
func setPrompt(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	prompt, reply, found := strings.Cut(strings.TrimSpace(args), "\n")
	if !found {
		// 兼容一行写法: 第一个空格前是触发词
		prompt, reply, found = strings.Cut(strings.TrimSpace(args), " ")
	}
	if !found {
		core.SendErrorMessage(bot, message.Chat.ID,
			"需要触发词和回复内容两部分。\n第一行写触发词，之后写回复内容。")
		return
	}

	prompt = strings.TrimSpace(prompt)
	reply = strings.TrimSpace(reply)

	if err := prompt_reply.SetPromptReply(prompt, reply); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("设置失败：%v", err))
		return
	}
	core.SendMessage(bot, message.Chat.ID,
		fmt.Sprintf("已设置：群里有人说到「%s」时，回复：\n%s", prompt, reply))
}

// deletePrompt 批量删除自动回复
func deletePrompt(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string) {
	var report batchReport

	for _, prompt := range splitLines(args) {
		if err := prompt_reply.DeletePromptReply(prompt); err != nil {
			report.failed = append(report.failed, fmt.Sprintf("%s（%v）", prompt, err))
			log.Printf("[Command] 删除自动回复 %q 失败: %v", prompt, err)
			continue
		}
		report.succeeded = append(report.succeeded, prompt)
	}

	core.SendMessage(bot, message.Chat.ID, report.render("已删除", "未找到"))
}

// listPrompts 列出全部自动回复
func listPrompts(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	replies := prompt_reply.All()
	if len(replies) == 0 {
		core.SendMessage(bot, message.Chat.ID, "还没有设置任何自动回复。")
		return
	}

	items := make([]string, 0, len(replies))
	for prompt, reply := range replies {
		items = append(items, fmt.Sprintf("%s → %s", prompt, reply))
	}

	if err := core.SendLongMessage(bot, message.Chat.ID,
		fmt.Sprintf("自动回复（%d 条）：", len(items)), items); err != nil {
		core.SendErrorMessage(bot, message.Chat.ID, "发送列表时发生错误。")
	}
}
