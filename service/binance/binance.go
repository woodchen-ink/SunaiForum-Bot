package binance

//币安价格推送
import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/woodchen-ink/Q58Bot/core"
)

var (
	botToken    string
	chatID      int64
	symbols     []string
	bot         *tgbotapi.BotAPI
	lastMsgID   int
	singaporeTZ *time.Location
)

type tickerInfo struct {
	symbol        string
	last          float64
	changePercent float64
}

func getTickerInfo(symbol string) (tickerInfo, error) {
	client := binance.NewClient("", "")

	ticker, err := client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return tickerInfo{}, err
	}
	if len(ticker) == 0 {
		return tickerInfo{}, fmt.Errorf("no ticker found for symbol %s", symbol)
	}
	last, err := strconv.ParseFloat(ticker[0].Price, 64)
	if err != nil {
		return tickerInfo{}, err
	}

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
	now := time.Now().In(singaporeTZ)
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
		log.Printf("Failed to send message. Error: %v\nFull message content:\nChat ID: %d\nMessage: %s", err, chatID, message)
		return
	}

	lastMsgID = sentMsg.MessageID
}

func RunBinance() {
	log.Println("Starting Binance service...")

	// 初始化必要的变量
	botToken = core.BOT_TOKEN
	bot = core.Bot
	chatID = core.ChatID
	symbols = core.Symbols
	singaporeTZ = core.SingaporeTZ

	// 初始化并加载所有交易对
	if err := LoadAllSymbols(); err != nil {
		log.Fatalf("Failed to load all trading pairs: %v", err)
	}

	// 启动每小时刷新交易对缓存
	go StartSymbolRefresh(1 * time.Hour)

	// 立即发送一次价格更新
	sendPriceUpdate()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now().In(singaporeTZ)
		if now.Minute() == 0 {
			log.Println("Sending hourly price update...")
			sendPriceUpdate()
		}
	}
}
