package core

//核心函数
import (
	"fmt"
	"log"
	"strconv"
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
			log.Printf("删除消息失败 (ChatID: %d, MessageID: %d): %v", chatID, messageID, err)
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
