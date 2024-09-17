package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/woodchen-ink/Q58Bot/service"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	BOT_TOKEN string
	ADMIN_ID  int64
)

func init() {
	// 设置时区
	setTimeZone()

	// 其他初始化逻辑
	initializeVariables()
}

func setTimeZone() {
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc
}

func initializeVariables() {
	BOT_TOKEN = os.Getenv("BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID")
	var err error
	ADMIN_ID, err = strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid ADMIN_ID: %v", err)
	}
}

func setupBot() {
	bot, err := tgbotapi.NewBotAPI(BOT_TOKEN)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.Chat.ID != ADMIN_ID {
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		switch update.Message.Command() {
		case "start":
			msg.Text = "Hello! I'm your bot."
		case "help":
			msg.Text = "I can help you with various tasks."
		default:
			msg.Text = "I don't know that command"
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
	}
}

func runGuard() {
	for {
		try(func() {
			service.RunGuard()
		}, "Guard")
	}
}

func runBinance() {
	for {
		try(func() {
			service.RunBinance()
		}, "Binance")
	}
}

func try(fn func(), name string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("%s process crashed: %v", name, r)
			log.Printf("Restarting %s process...", name)
			time.Sleep(time.Second) // 添加短暂延迟以防止过快重启
		}
	}()
	fn()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 使用 goroutines 运行 bot、guard 和 binance 服务
	go setupBot()
	go runGuard()
	go runBinance()

	// 保持主程序运行
	select {}
}
