package service

import (
	"fmt"
	"strings"
	"sync"

	"github.com/woodchen-ink/Q58Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	promptReplies = make(map[string]string)
	promptMutex   sync.RWMutex
)

func SetPromptReply(prompt, reply string) {
	promptMutex.Lock()
	defer promptMutex.Unlock()
	promptReplies[strings.ToLower(prompt)] = reply
}

func DeletePromptReply(prompt string) {
	promptMutex.Lock()
	defer promptMutex.Unlock()
	delete(promptReplies, strings.ToLower(prompt))
}

func GetPromptReply(message string) (string, bool) {
	promptMutex.RLock()
	defer promptMutex.RUnlock()
	for prompt, reply := range promptReplies {
		if strings.Contains(strings.ToLower(message), prompt) {
			return reply, true
		}
	}
	return "", false
}

func ListPromptReplies() string {
	promptMutex.RLock()
	defer promptMutex.RUnlock()

	if len(promptReplies) == 0 {
		return "目前没有设置任何提示词回复。"
	}

	var result strings.Builder
	result.WriteString("当前设置的提示词回复：\n")
	for prompt, reply := range promptReplies {
		result.WriteString(fmt.Sprintf("提示词: %s\n回复: %s\n\n", prompt, reply))
	}
	return result.String()
}

func HandlePromptCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !core.IsAdmin(message.From.ID) {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "只有管理员才能使用此命令。"))
		return
	}

	args := strings.SplitN(message.Text, " ", 3)
	if len(args) < 2 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "使用方法: /prompt set <提示词> <回复>\n/prompt delete <提示词>\n/prompt list"))
		return
	}

	switch args[1] {
	case "set":
		if len(args) < 3 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "使用方法: /prompt set <提示词> <回复>"))
			return
		}
		promptAndReply := strings.SplitN(args[2], " ", 2)
		if len(promptAndReply) < 2 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "请同时提供提示词和回复。"))
			return
		}
		SetPromptReply(promptAndReply[0], promptAndReply[1])
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已设置提示词 '%s' 的回复。", promptAndReply[0])))
	case "delete":
		if len(args) < 3 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "使用方法: /prompt delete <提示词>"))
			return
		}
		DeletePromptReply(args[2])
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已删除提示词 '%s' 的回复。", args[2])))
	case "list":
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, ListPromptReplies()))
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "未知的子命令。使用方法: /prompt set|delete|list"))
	}
}

func CheckAndReplyPrompt(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if reply, found := GetPromptReply(message.Text); found {
		replyMsg := tgbotapi.NewMessage(message.Chat.ID, reply)
		replyMsg.ReplyToMessageID = message.MessageID
		bot.Send(replyMsg)
	}
}
