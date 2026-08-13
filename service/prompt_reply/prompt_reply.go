package prompt_reply

// 关键词触发的自动回复。
// 提示词全量常驻内存: 数据量小且每条群消息都要匹配, 启动时加载一次, 增删时同步更新内存与库。
import (
	"fmt"
	"log"
	"strings"
	"sync"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type PromptReplyManager struct {
	mu            sync.RWMutex
	promptReplies map[string]string // 提示词(小写) -> 回复
}

var Manager = &PromptReplyManager{
	promptReplies: make(map[string]string),
}

// LoadDataFromDatabase 全量重载内存映射, 启动时调用
func (prm *PromptReplyManager) LoadDataFromDatabase() error {
	promptReplies, err := core.DB.GetAllPromptReplies()
	if err != nil {
		return err
	}

	prm.mu.Lock()
	prm.promptReplies = promptReplies
	prm.mu.Unlock()

	log.Printf("[PromptReply] 已从数据库加载 %d 条提示回复", len(promptReplies))
	return nil
}

// snapshot 返回内存映射的副本, 供只读遍历使用
func (prm *PromptReplyManager) snapshot() map[string]string {
	prm.mu.RLock()
	defer prm.mu.RUnlock()

	result := make(map[string]string, len(prm.promptReplies))
	for k, v := range prm.promptReplies {
		result[k] = v
	}
	return result
}

// SetPromptReply 新增或覆盖一条提示词回复, 先落库再更新内存, 保证重启后一致
func SetPromptReply(prompt, reply string) error {
	if err := core.ValidatePrompt(prompt, reply); err != nil {
		return fmt.Errorf("输入验证失败: %w", err)
	}

	prompt = strings.ToLower(strings.TrimSpace(prompt))
	reply = strings.TrimSpace(reply)

	if err := core.DB.AddPromptReply(prompt, reply); err != nil {
		log.Printf("[PromptReply] 设置提示回复失败: %v", err)
		return err
	}

	Manager.mu.Lock()
	Manager.promptReplies[prompt] = reply
	count := len(Manager.promptReplies)
	Manager.mu.Unlock()

	log.Printf("[PromptReply] 设置提示回复成功, 当前数量: %d", count)
	return nil
}

// DeletePromptReply 删除一条提示词回复, 先落库再更新内存
func DeletePromptReply(prompt string) error {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return fmt.Errorf("提示词不能为空")
	}

	if err := core.DB.DeletePromptReply(prompt); err != nil {
		log.Printf("[PromptReply] 删除提示回复失败: %v", err)
		return err
	}

	Manager.mu.Lock()
	delete(Manager.promptReplies, prompt)
	count := len(Manager.promptReplies)
	Manager.mu.Unlock()

	log.Printf("[PromptReply] 删除提示回复成功, 当前数量: %d", count)
	return nil
}

// GetPromptReply 在消息中查找命中的提示词。
// 多个提示词同时命中时取最长的那个, 保证结果稳定且更具体的规则优先。
func GetPromptReply(message string) (string, bool) {
	message = strings.ToLower(message)

	var bestPrompt, bestReply string
	Manager.mu.RLock()
	for prompt, reply := range Manager.promptReplies {
		if len(prompt) > len(bestPrompt) && strings.Contains(message, prompt) {
			bestPrompt, bestReply = prompt, reply
		}
	}
	Manager.mu.RUnlock()

	return bestReply, bestPrompt != ""
}

// ListPromptReplies 渲染全部提示词回复供管理员查看
func ListPromptReplies() string {
	replies := Manager.snapshot()
	if len(replies) == 0 {
		return "没有找到提示回复"
	}

	var result strings.Builder
	for prompt, reply := range replies {
		fmt.Fprintf(&result, "Prompt: %s\nReply: %s\n\n", prompt, reply)
	}
	return result.String()
}

// HandlePromptCommand 处理 /prompt set|delete|list 子命令
func HandlePromptCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if !core.IsAdmin(message.From.ID) {
		core.SendMessage(bot, message.Chat.ID, "只有管理员才能使用此命令。")
		return
	}

	const usage = "使用方法:\n/prompt set <提示词> <回复>\n/prompt delete <提示词>\n/prompt list"

	args := strings.SplitN(message.Text, " ", 3)
	if len(args) < 2 {
		core.SendMessage(bot, message.Chat.ID, usage)
		return
	}

	switch args[1] {
	case "set":
		if len(args) < 3 {
			core.SendMessage(bot, message.Chat.ID, "使用方法: /prompt set <提示词> <回复>")
			return
		}
		promptAndReply := strings.SplitN(args[2], " ", 2)
		if len(promptAndReply) < 2 {
			core.SendMessage(bot, message.Chat.ID, "请同时提供提示词和回复。")
			return
		}
		if err := SetPromptReply(promptAndReply[0], promptAndReply[1]); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("设置提示词失败：%v", err))
			return
		}
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("已设置提示词 '%s' 的回复。", promptAndReply[0]))
	case "delete":
		if len(args) < 3 {
			core.SendMessage(bot, message.Chat.ID, "使用方法: /prompt delete <提示词>")
			return
		}
		if err := DeletePromptReply(args[2]); err != nil {
			core.SendErrorMessage(bot, message.Chat.ID, fmt.Sprintf("删除提示词失败：%v", err))
			return
		}
		core.SendMessage(bot, message.Chat.ID, fmt.Sprintf("已删除提示词 '%s' 的回复。", args[2]))
	case "list":
		core.SendMessage(bot, message.Chat.ID, ListPromptReplies())
	default:
		core.SendMessage(bot, message.Chat.ID, usage)
	}
}

// CheckAndReplyPrompt 群消息命中提示词时以引用方式回复
func CheckAndReplyPrompt(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	reply, found := GetPromptReply(message.Text)
	if !found {
		return
	}

	replyMsg := tgbotapi.NewMessage(message.Chat.ID, reply)
	replyMsg.ReplyToMessageID = message.MessageID
	if _, err := bot.Send(replyMsg); err != nil {
		log.Printf("[PromptReply] 发送自动回复失败: %v", err)
	}
}
