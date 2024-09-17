package service

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/woodchen-ink/Q58Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	adminID   int64
	dbFile    string
	debugMode bool
)

func init() {
	botToken = os.Getenv("BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID")
	var err error
	adminID, err = strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid ADMIN_ID: %v", err)
	}
	dbFile = "/app/data/q58.db" // 新的数据库文件路径
	debugMode = os.Getenv("DEBUG_MODE") == "true"
}

type RateLimiter struct {
	mu       sync.Mutex
	maxCalls int
	period   time.Duration
	calls    []time.Time
}

func NewRateLimiter(maxCalls int, period time.Duration) *RateLimiter {
	return &RateLimiter{
		maxCalls: maxCalls,
		period:   period,
		calls:    make([]time.Time, 0, maxCalls),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if len(r.calls) < r.maxCalls {
		r.calls = append(r.calls, now)
		return true
	}

	if now.Sub(r.calls[0]) >= r.period {
		r.calls = append(r.calls[1:], now)
		return true
	}

	return false
}

func deleteMessageAfterDelay(bot *tgbotapi.BotAPI, chatID int64, messageID int, delay time.Duration) {
	time.Sleep(delay)
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	_, err := bot.Request(deleteMsg)
	if err != nil {
		log.Printf("Failed to delete message: %v", err)
	}
}

func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, linkFilter *core.LinkFilter) {
	if message.Chat.Type != "private" {
		log.Printf("Processing message: %s", message.Text)
		shouldFilter, newLinks := linkFilter.ShouldFilter(message.Text)
		if shouldFilter {
			log.Printf("Message should be filtered: %s", message.Text)
			if message.From.ID != adminID {
				deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
				_, err := bot.Request(deleteMsg)
				if err != nil {
					log.Printf("Failed to delete message: %v", err)
				}
				notification := tgbotapi.NewMessage(message.Chat.ID, "已撤回该消息。注:一个链接不能发两次.")
				sent, err := bot.Send(notification)
				if err != nil {
					log.Printf("Failed to send notification: %v", err)
				} else {
					go deleteMessageAfterDelay(bot, message.Chat.ID, sent.MessageID, 3*time.Minute)
				}
			}
			return
		}
		if len(newLinks) > 0 {
			log.Printf("New non-whitelisted links found: %v", newLinks)
		}
	}
}

func StartBot() error {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = debugMode

	log.Printf("Authorized on account %s", bot.Self.UserName)

	err = core.RegisterCommands(bot)
	if err != nil {
		return fmt.Errorf("error registering commands: %w", err)
	}

	linkFilter, err := core.NewLinkFilter(dbFile)
	if err != nil {
		log.Fatalf("Failed to create LinkFilter: %v", err)
	}

	rateLimiter := NewRateLimiter(10, time.Second)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		go handleUpdate(bot, update, linkFilter, rateLimiter)
	}

	return nil
}
func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, linkFilter *core.LinkFilter, rateLimiter *RateLimiter) {
	if update.Message == nil {
		return
	}

	// 检查是否是管理员发送的私聊消息
	if update.Message.Chat.Type == "private" && update.Message.From.ID == adminID {
		command := update.Message.Command()
		args := update.Message.CommandArguments()

		switch command {
		case "add", "delete", "list", "deletecontaining":
			linkFilter.HandleKeywordCommand(bot, update.Message, command, args)
		case "addwhite", "delwhite", "listwhite":
			linkFilter.HandleWhitelistCommand(bot, update.Message, command, args)
		default:
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "未知命令")
			bot.Send(msg)
		}
		return
	}

	// 处理非管理员消息或群组消息
	if update.Message.Chat.Type != "private" {
		if rateLimiter.Allow() {
			processMessage(bot, update.Message, linkFilter)
		}
	}
}

func RunGuard() {
	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	delay := baseDelay

	for {
		err := StartBot()
		if err != nil {
			log.Printf("Bot encountered an error: %v", err)
			log.Printf("Attempting to restart in %v...", delay)
			time.Sleep(delay)

			// 实现指数退避
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			// 如果 bot 正常退出，重置延迟
			delay = baseDelay
			log.Println("Bot disconnected. Attempting to restart immediately...")
		}
	}
}
