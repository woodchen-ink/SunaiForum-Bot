package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/woodchen-ink/Q58Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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

func RunGuard() {
	baseDelay := time.Second
	maxDelay := 5 * time.Minute
	delay := baseDelay

	for {
		err := startBot()
		if err != nil {
			log.Printf("Bot encountered an error: %v", err)
			log.Printf("Attempting to restart in %v...", delay)
			time.Sleep(delay)

			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		} else {
			delay = baseDelay
			log.Println("Bot disconnected. Attempting to restart immediately...")
		}
	}
}

func startBot() error {
	bot, err := tgbotapi.NewBotAPI(core.BOT_TOKEN)
	if err != nil {
		return fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = debugMode

	log.Printf("Authorized on account %s", bot.Self.UserName)

	err = core.RegisterCommands(bot)
	if err != nil {
		return fmt.Errorf("error registering commands: %w", err)
	}

	linkFilter, err := NewLinkFilter(dbFile)
	if err != nil {
		return fmt.Errorf("failed to create LinkFilter: %v", err)
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

func handleUpdate(bot *tgbotapi.BotAPI, update tgbotapi.Update, linkFilter *LinkFilter, rateLimiter *RateLimiter) {
	if update.Message == nil {
		return
	}

	if update.Message.Chat.Type == "private" && update.Message.From.ID == core.ADMIN_ID {
		handleAdminCommand(bot, update.Message, linkFilter)
		return
	}

	if update.Message.Chat.Type != "private" && rateLimiter.Allow() {
		processMessage(bot, update.Message, linkFilter)
	}
}

func handleAdminCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, linkFilter *LinkFilter) {
	command := message.Command()
	args := message.CommandArguments()

	switch command {
	case "add", "delete", "list", "deletecontaining":
		linkFilter.HandleKeywordCommand(bot, message, command, args)
	case "addwhite", "delwhite", "listwhite":
		linkFilter.HandleWhitelistCommand(bot, message, command, args)
	case "prompt":
		HandlePromptCommand(bot, message)
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "未知命令"))
	}
}

func processMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message, linkFilter *LinkFilter) {
	log.Printf("Processing message: %s", message.Text)
	shouldFilter, newLinks := linkFilter.ShouldFilter(message.Text)
	if shouldFilter {
		log.Printf("Message should be filtered: %s", message.Text)
		if message.From.ID != core.ADMIN_ID {
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

	CheckAndReplyPrompt(bot, message)
}
