package service

// 消息处理辅助函数
import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
)

// deleteMessageAfterDelay 根据指定延迟删除消息。
func deleteMessageAfterDelay(bot *tgbotapi.BotAPI, chatID int64, messageID int, delay time.Duration) {
	// 让线程暂停指定的延迟时间。
	time.Sleep(delay)

	// 创建一个删除消息的请求。
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)

	// 尝试发送删除消息的请求，并检查是否有错误发生。
	// 注意: 错误情况下只是记录错误，不进行其他操作。
	_, err := bot.Request(deleteMsg)
	if err != nil {
		log.Printf("删除消息失败: %v", err)
	}
}

// SendLongMessage 如有必要，可将长消息拆分为多条消息来发送
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

// HandleKeywordCommand 处理关键词命令
func HandleKeywordCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string, db *core.Database) {
	switch command {
	case "list":
		keywords, err := db.GetAllKeywords()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "获取关键词列表时发生错误。"))
			return
		}
		if len(keywords) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "关键词列表为空。"))
		} else {
			SendLongMessage(bot, message.Chat.ID, "当前关键词列表：", keywords)
		}
	case "add":
		if args != "" {
			keyword := args
			exists, err := db.KeywordExists(keyword)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "检查关键词时发生错误。"))
				return
			}
			if !exists {
				err = db.AddKeyword(keyword)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "添加关键词时发生错误。"))
				} else {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword)))
				}
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword)))
			}
		}
	case "delete":
		if args != "" {
			keyword := args
			err := db.RemoveKeyword(keyword)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("删除关键词 '%s' 时发生错误: %v", keyword, err)))
				return
			}

			// 检查关键词是否仍然存在
			exists, err := db.KeywordExists(keyword)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查关键词 '%s' 是否存在时发生错误: %v", keyword, err)))
				return
			}

			if !exists {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已成功删除。", keyword)))
			} else {
				similarKeywords, err := db.SearchKeywords(keyword)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "搜索关键词时发生错误。"))
					return
				}
				if len(similarKeywords) > 0 {
					SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
				} else {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("未能删除关键词 '%s'，且未找到相似的关键词。", keyword)))
				}
			}
		} else {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "请提供要删除的关键词。"))
		}
	case "deletecontaining":
		if args != "" {
			substring := args
			removedKeywords, err := db.RemoveKeywordsContaining(substring)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "删除关键词时发生错误。"))
				return
			}
			if len(removedKeywords) > 0 {
				SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removedKeywords)
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring)))
			}
		}
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "无效的命令或参数。"))
	}
}

func HandleWhitelistCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string, db *core.Database) {
	switch command {
	case "listwhite":
		whitelist, err := db.GetAllWhitelist()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("获取白名单时发生错误: %v", err)))
			return
		}
		if len(whitelist) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "白名单为空。"))
		} else {
			SendLongMessage(bot, message.Chat.ID, "白名单域名列表：", whitelist)
		}

	case "addwhite":
		if args == "" {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "请提供要添加的域名。"))
			return
		}
		domain := strings.ToLower(args)
		exists, err := db.WhitelistExists(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err)))
			return
		}
		if exists {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain)))
			return
		}
		err = db.AddWhitelist(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("添加到白名单时发生错误: %v", err)))
			return
		}
		// 再次检查以确保添加成功
		exists, err = db.WhitelistExists(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("验证添加操作时发生错误: %v", err)))
			return
		}
		if exists {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功添加到白名单。", domain)))
		} else {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("未能添加域名 '%s' 到白名单。", domain)))
		}

	case "delwhite":
		if args == "" {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "请提供要删除的域名。"))
			return
		}
		domain := strings.ToLower(args)
		exists, err := db.WhitelistExists(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("检查白名单时发生错误: %v", err)))
			return
		}
		if !exists {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain)))
			return
		}
		err = db.RemoveWhitelist(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("从白名单删除时发生错误: %v", err)))
			return
		}
		// 再次检查以确保删除成功
		exists, err = db.WhitelistExists(domain)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("验证删除操作时发生错误: %v", err)))
			return
		}
		if !exists {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已成功从白名单中删除。", domain)))
		} else {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("未能从白名单中删除域名 '%s'。", domain)))
		}

	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "无效的命令或参数。"))
	}
}
