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
