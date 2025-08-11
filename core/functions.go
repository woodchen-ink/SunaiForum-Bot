package core

//核心函数
import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func IsAdmin(userID int64) bool {
	return userID == ADMIN_ID
}
func mustParseInt64(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}

	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("未能将'%s'解析为 int64: %v", s, err)
	}

	return value, nil
}
func DeleteMessageAfterDelay(bot *tgbotapi.BotAPI, chatID int64, messageID int, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Printf("DeleteMessageAfterDelay: 删除消息失败 (ChatID: %d, MessageID: %d): %v", chatID, messageID, err)
		}
	}()
}

func SendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	return err
}

func SendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, errMsg string) {
	SendMessage(bot, chatID, errMsg)
}

const (
	maxMessageLength = 4000
)

func SendLongMessage(bot *tgbotapi.BotAPI, chatID int64, prefix string, items []string) error {
	message := prefix + "\n"
	for i, item := range items {
		newLine := fmt.Sprintf("%d. %s\n", i+1, item)
		if len(message)+len(newLine) > maxMessageLength {
			if err := SendMessage(bot, chatID, message); err != nil {
				return err
			}
			message = ""
		}
		message += newLine
	}

	if message != "" {
		return SendMessage(bot, chatID, message)
	}

	return nil
}

// 输入验证函数
func ValidateKeyword(keyword string) error {
	if keyword == "" {
		return fmt.Errorf("关键词不能为空")
	}
	
	keyword = strings.TrimSpace(keyword)
	if len(keyword) == 0 {
		return fmt.Errorf("关键词不能为空白字符")
	}
	
	if len(keyword) > 100 {
		return fmt.Errorf("关键词长度不能超过100个字符")
	}
	
	// 防止SQL注入和特殊字符
	if strings.ContainsAny(keyword, "';\"\\") {
		return fmt.Errorf("关键词包含不允许的字符")
	}
	
	return nil
}

func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}
	
	domain = strings.TrimSpace(strings.ToLower(domain))
	if len(domain) == 0 {
		return fmt.Errorf("域名不能为空白字符")
	}
	
	if len(domain) > 253 {
		return fmt.Errorf("域名长度不能超过253个字符")
	}
	
	// 简单的域名格式验证
	domainPattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("域名格式无效")
	}
	
	return nil
}

func ValidatePrompt(prompt, reply string) error {
	if prompt == "" {
		return fmt.Errorf("提示词不能为空")
	}
	
	if reply == "" {
		return fmt.Errorf("回复内容不能为空")
	}
	
	prompt = strings.TrimSpace(prompt)
	reply = strings.TrimSpace(reply)
	
	if len(prompt) == 0 || len(reply) == 0 {
		return fmt.Errorf("提示词和回复内容不能为空白字符")
	}
	
	if len(prompt) > 100 {
		return fmt.Errorf("提示词长度不能超过100个字符")
	}
	
	if len(reply) > 1000 {
		return fmt.Errorf("回复内容长度不能超过1000个字符")
	}
	
	// 防止SQL注入
	if strings.ContainsAny(prompt, "';\"\\") || strings.ContainsAny(reply, "';\"\\") {
		return fmt.Errorf("提示词和回复内容包含不允许的字符")
	}
	
	return nil
}
