package core

// Telegram 消息收发的共用封装
import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// maxMessageLength Telegram 单条消息上限 4096, 留出余量避免分段边界超限
const maxMessageLength = 4000

func SendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	return err
}

// SendErrorMessage 发送面向用户的错误提示, 与普通消息区分调用点语义
func SendErrorMessage(bot *tgbotapi.BotAPI, chatID int64, errMsg string) {
	if err := SendMessage(bot, chatID, "❌ "+errMsg); err != nil {
		log.Printf("[Core] 发送错误提示失败 (ChatID: %d): %v", chatID, err)
	}
}

// SendLongMessage 把带编号的列表按长度上限自动分段发送
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

// DeleteMessageAfterDelay 异步延时删除消息, 用于机器人自身的临时提示
func DeleteMessageAfterDelay(bot *tgbotapi.BotAPI, chatID int64, messageID int, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
		if _, err := bot.Request(deleteMsg); err != nil {
			log.Printf("[Core] 延时删除消息失败 (ChatID: %d, MessageID: %d): %v", chatID, messageID, err)
		}
	}()
}

// DeleteMessages 立即批量删除消息, 逐条失败只记录不中断
func DeleteMessages(bot *tgbotapi.BotAPI, chatID int64, messageIDs ...int) {
	for _, msgID := range messageIDs {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, msgID)
		if _, err := bot.Request(deleteMsg); err != nil {
			log.Printf("[Core] 删除消息 %d 失败 (ChatID: %d): %v", msgID, chatID, err)
		}
	}
}
