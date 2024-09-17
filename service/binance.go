package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/adshao/go-binance/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	botToken    string
	chatID      int64
	symbols     []string
	bot         *tgbotapi.BotAPI
	lastMsgID   int
	singaporeTZ *time.Location
)

func init() {
	var err error
	botToken = os.Getenv("BOT_TOKEN")
	chatID = mustParseInt64(os.Getenv("CHAT_ID"))
	symbols = strings.Split(os.Getenv("SYMBOLS"), ",")

	// 初始化 singaporeTZ
	singaporeTZ = time.FixedZone("Asia/Singapore", 8*60*60) // UTC+8

	bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}
}

func mustParseInt64(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		log.Fatal(err)
	}
	return i
}

type tickerInfo struct {
	symbol        string
	last          float64
	changePercent float64
}

func getTickerInfo(symbol string) (tickerInfo, error) {
	client := binance.NewClient("", "")

	// 获取当前价格
	ticker, err := client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return tickerInfo{}, err
	}
	if len(ticker) == 0 {
		return tickerInfo{}, fmt.Errorf("no ticker found for symbol %s", symbol)
	}
	// 在 getTickerInfo 函数中
	last, err := strconv.ParseFloat(ticker[0].Price, 64)
	if err != nil {
		return tickerInfo{}, err
	}

	// 获取24小时价格变化
	stats, err := client.NewListPriceChangeStatsService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return tickerInfo{}, err
	}
	if len(stats) == 0 {
		return tickerInfo{}, fmt.Errorf("no price change stats found for symbol %s", symbol)
	}
	changePercent, err := strconv.ParseFloat(stats[0].PriceChangePercent, 64)
	if err != nil {
		return tickerInfo{}, err
	}

	return tickerInfo{
		symbol:        symbol,
		last:          last,
		changePercent: changePercent,
	}, nil
}

func formatChange(changePercent float64) string {
	if changePercent > 0 {
		return fmt.Sprintf("🔼 +%.2f%%", changePercent)
	} else if changePercent < 0 {
		return fmt.Sprintf("🔽 %.2f%%", changePercent)
	}
	return fmt.Sprintf("◀▶ %.2f%%", changePercent)
}

func sendPriceUpdate() {
	var now time.Time
	if singaporeTZ != nil {
		now = time.Now().In(singaporeTZ)
	} else {
		now = time.Now().UTC()
		log.Println("Warning: singaporeTZ is nil, using UTC")
	}
	message := fmt.Sprintf("市场更新 - %s (SGT)\n\n", now.Format("2006-01-02 15:04:05"))

	for _, symbol := range symbols {
		info, err := getTickerInfo(symbol)
		if err != nil {
			log.Printf("Error getting ticker info for %s: %v", symbol, err)
			continue
		}

		changeStr := formatChange(info.changePercent)

		message += fmt.Sprintf("*%s*\n", info.symbol)
		message += fmt.Sprintf("价格: $%.7f\n", info.last)
		message += fmt.Sprintf("24h 涨跌: %s\n\n", changeStr)
	}

	if lastMsgID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, lastMsgID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Printf("Failed to delete previous message: %v", err)
		}
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
		return
	}

	lastMsgID = sentMsg.MessageID
}

func RunBinance() {
	log.Println("Starting Binance service...")
	for {
		log.Println("Sending price update...")
		sendPriceUpdate()

		log.Println("Waiting for next update...")
		time.Sleep(1 * time.Hour)
	}
}
