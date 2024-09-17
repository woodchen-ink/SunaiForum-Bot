package core

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RegisterCommands(bot *tgbotapi.BotAPI, adminID int64) error {
	commands := []tgbotapi.BotCommand{
		{Command: "add", Description: "添加新的关键词"},
		{Command: "delete", Description: "删除现有的关键词"},
		{Command: "list", Description: "列出所有当前的关键词"},
		{Command: "deletecontaining", Description: "删除所有包含指定词语的关键词"},
		{Command: "addwhite", Description: "添加域名到白名单"},
		{Command: "delwhite", Description: "从白名单移除域名"},
		{Command: "listwhite", Description: "列出白名单域名"},
	}

	scope := tgbotapi.NewBotCommandScopeChatAdministrators(adminID)

	config := tgbotapi.NewSetMyCommands(commands...)
	config.Scope = &scope    // 注意这里使用 &scope 来获取指针
	config.LanguageCode = "" // 空字符串表示默认语言

	_, err := bot.Request(config)
	if err != nil {
		return fmt.Errorf("failed to register bot commands: %w", err)
	}

	log.Println("Bot commands registered successfully.")
	return nil
}
