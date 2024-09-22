package group_member_management

import (
	"fmt"
	"log"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
)

var logger = log.New(log.Writer(), "GroupMemberManagement: ", log.Ldate|log.Ltime|log.Lshortfile)

func HandleBanCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// 检查是否是管理员
	if !core.IsAdmin(message.From.ID) {
		return
	}

	// 检查是否是回复消息
	if message.ReplyToMessage == nil {
		return
	}

	chatID := message.Chat.ID
	userToBan := message.ReplyToMessage.From

	// 立即删除被回复的原消息
	deleteConfig := tgbotapi.NewDeleteMessage(chatID, message.ReplyToMessage.MessageID)
	_, err := bot.Request(deleteConfig)
	if err != nil {
		logger.Printf("删除原消息时出错: %v", err)
	}

	// 踢出用户
	kickChatMemberConfig := tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userToBan.ID,
		},
		UntilDate: 0, // 0 means ban forever
	}

	_, err = bot.Request(kickChatMemberConfig)
	if err != nil {
		logger.Printf("禁止用户时出错: %v", err)
		return
	}

	// 发送提示消息
	banMessage := fmt.Sprintf("用户 %s 已被封禁并踢出群组。", userToBan.UserName)
	msg := tgbotapi.NewMessage(chatID, banMessage)
	sentMsg, err := bot.Send(msg)
	if err != nil {
		logger.Printf("发送禁止消息时出错: %v", err)
		return
	}

	// 3分钟后删除机器人的消息和管理员的指令消息
	go deleteMessagesAfterDelay(bot, chatID, []int{sentMsg.MessageID, message.MessageID}, 3*time.Minute)
}

func deleteMessagesAfterDelay(bot *tgbotapi.BotAPI, chatID int64, messageIDs []int, delay time.Duration) {
	time.Sleep(delay)
	for _, msgID := range messageIDs {
		deleteConfig := tgbotapi.NewDeleteMessage(chatID, msgID)
		_, err := bot.Request(deleteConfig)
		if err != nil {
			logger.Printf("删除消息 %d 时出错: %v", msgID, err)
		}
	}
}
