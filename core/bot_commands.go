package core

// 注册命令
import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RegisterCommands(bot *tgbotapi.BotAPI) error {
	commands := []tgbotapi.BotCommand{
		{Command: "add", Description: "添加新的关键词"},
		{Command: "delete", Description: "删除现有的关键词"},
		{Command: "list", Description: "列出所有当前的关键词"},
		{Command: "deletecontaining", Description: "删除所有包含指定词语的关键词"},
		{Command: "addwhite", Description: "添加域名到白名单"},
		{Command: "delwhite", Description: "从白名单移除域名"},
		{Command: "listwhite", Description: "列出白名单域名"},
		{Command: "prompt", Description: "设置提示词回复"},
	}

	config := tgbotapi.NewSetMyCommands(commands...)

	// 不设置 Scope，这将使命令对所有用户可见
	// config.Scope = nil

	config.LanguageCode = "" // 空字符串表示默认语言

	_, err := bot.Request(config)
	if err != nil {
		return fmt.Errorf("注册机器人命令失败: %w", err)
	}

	log.Println("机器人命令注册成功。")
	return nil
}
