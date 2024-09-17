package core

import (
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendLongMessage sends a long message by splitting it into multiple messages if necessary
// SendLongMessage sends a long message by splitting it into multiple messages if necessary
func SendLongMessage(bot *tgbotapi.BotAPI, chatID int64, prefix string, items []string) error {
	const maxMessageLength = 4000 // Leave some room for Telegram's message limit

	message := prefix + "\n"
	for i, item := range items {
			newLine := fmt.Sprintf("%d. %s\n", i+1, item)
			if len(message)+len(newLine) > maxMessageLength {
					msg := tgbotapi.NewMessage(chatID, message)
					_, err := bot.Send(msg)
					if err != nil {
							return err
					}
					message = ""
			}
			message += newLine
	}

	if message != "" {
			msg := tgbotapi.NewMessage(chatID, message)
			_, err := bot.Send(msg)
			if err != nil {
					return err
			}
	}

	return nil
}

// SendLongMessageWithoutNumbering sends a long message without numbering the items
func SendLongMessageWithoutNumbering(bot *tgbotapi.BotAPI, chatID int64, prefix string, items []string) error {
	const maxMessageLength = 4000 // Leave some room for Telegram's message limit

	message := prefix + "\n"
	for _, item := range items {
			newLine := item + "\n"
			if len(message)+len(newLine) > maxMessageLength {
					msg := tgbotapi.NewMessage(chatID, message)
					_, err := bot.Send(msg)
					if err != nil {
							return err
					}
					message = ""
			}
			message += newLine
	}

	if message != "" {
			msg := tgbotapi.NewMessage(chatID, message)
			_, err := bot.Send(msg)
			if err != nil {
					return err
			}
	}

	return nil
}

// JoinLongMessage joins items into a single long message, splitting it if necessary
func JoinLongMessage(prefix string, items []string) []string {
	const maxMessageLength = 4000 // Leave some room for Telegram's message limit

	var messages []string
	message := prefix + "\n"

	for i, item := range items {
		newLine := fmt.Sprintf("%d. %s\n", i+1, item)
		if len(message)+len(newLine) > maxMessageLength {
			messages = append(messages, strings.TrimSpace(message))
			message = ""
		}
		message += newLine
	}

	if message != "" {
		messages = append(messages, strings.TrimSpace(message))
	}

	return messages
}
