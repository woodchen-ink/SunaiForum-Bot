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

// All 返回全部提示词回复的副本, 供管理员查看
func All() map[string]string {
	return Manager.snapshot()
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
