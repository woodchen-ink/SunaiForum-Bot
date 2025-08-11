package prompt_reply

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/woodchen-ink/SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var logger = log.New(log.Writer(), "PromptReply: ", log.Ldate|log.Ltime|log.Lshortfile)

type PromptReplyManager struct {
	promptReplies map[string]string
	mu            sync.RWMutex
}

var Manager *PromptReplyManager

func init() {
	Manager = &PromptReplyManager{
		promptReplies: make(map[string]string),
	}
}

func (prm *PromptReplyManager) LoadDataFromDatabase() error {
	prm.mu.Lock()
	defer prm.mu.Unlock()

	promptReplies, err := core.DB.GetAllPromptReplies()
	if err != nil {
		return err
	}

	prm.promptReplies = promptReplies

	logger.Printf("提示回复: 已从数据库加载 %d 条提示回复", len(prm.promptReplies))
	return nil
}
func SetPromptReply(prompt, reply string) error {
	err := core.DB.AddPromptReply(prompt, reply)
	if err != nil {
		logger.Printf("提示回复: %s 设置提示回复失败: %v", time.Now().Format("2006/01/02 15:04:05"), err)
		return err
	}

	Manager.mu.Lock()
	Manager.promptReplies[prompt] = reply
	Manager.mu.Unlock()

	logger.Printf("提示回复: %s 设置提示回复成功。当前提示回复数量: %d", time.Now().Format("2006/01/02 15:04:05"), len(Manager.promptReplies))
	return nil
}

func DeletePromptReply(prompt string) error {
	err := core.DB.DeletePromptReply(prompt)
	if err != nil {
		logger.Printf("提示回复: %s 删除提示回复失败: %v", time.Now().Format("2006/01/02 15:04:05"), err)
		return err
	}

	Manager.mu.Lock()
	delete(Manager.promptReplies, prompt)
	Manager.mu.Unlock()

	logger.Printf("提示回复: %s 删除提示回复成功。当前提示回复数量: %d", time.Now().Format("2006/01/02 15:04:05"), len(Manager.promptReplies))
	return nil
}

func GetPromptReply(message string) (string, bool) {
	promptReplies, err := core.DB.GetAllPromptReplies()
	if err != nil {
		logger.Printf("Error getting prompt replies: %v", err)
		return "", false
	}

	message = strings.ToLower(message)
	for prompt, reply := range promptReplies {
		if strings.Contains(message, strings.ToLower(prompt)) {
			return reply, true
		}
	}
	return "", false
}

func ListPromptReplies() string {
	replies, err := core.DB.GetAllPromptReplies()
	if err != nil {
		logger.Printf("获取及时回复时出错: %v", err)
		return "检索提示回复时出错"
	}

	if len(replies) == 0 {
		return "没有找到提示回复"
	}

	var result strings.Builder
	for prompt, reply := range replies {
		result.WriteString(fmt.Sprintf("Prompt: %s\nReply: %s\n\n", prompt, reply))
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
		err := SetPromptReply(promptAndReply[0], promptAndReply[1])
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("设置提示词失败：%v", err)))
			return
		}
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("已设置提示词 '%s' 的回复。", promptAndReply[0])))
	case "delete":
		if len(args) < 3 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "使用方法: /prompt delete <提示词>"))
			return
		}
		err := DeletePromptReply(args[2])
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("删除提示词失败：%v", err)))
			return
		}
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
