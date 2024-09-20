package binance

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/adshao/go-binance/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	symbolsMu  sync.RWMutex
	allSymbols []string
)

// LoadSymbols 初始化并缓存所有交易对
func LoadAllSymbols() error {
	client := binance.NewClient("", "")
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return err
	}

	symbolsMu.Lock()
	defer symbolsMu.Unlock()

	allSymbols = nil // 清空旧的符号列表
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" && symbol.QuoteAsset == "USDT" {
			allSymbols = append(allSymbols, symbol.Symbol)
		}
	}

	log.Printf("Loaded %d valid USDT trading pairs", len(allSymbols))
	return nil
}

func GetAllSymbols() []string {
	symbolsMu.RLock()
	defer symbolsMu.RUnlock()
	return allSymbols
}

func StartSymbolRefresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Println("Refreshing trading pairs...")
			if err := LoadAllSymbols(); err != nil {
				log.Printf("Failed to refresh symbols: %v", err)
			}
		}
	}()
}

func HandleSymbolQuery(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	allSymbols := GetAllSymbols()
	msg := strings.TrimSpace(message.Text)

	for _, symbol := range allSymbols {
		coinName := strings.TrimSuffix(symbol, "USDT")
		if strings.EqualFold(msg, coinName) {
			info, err := getTickerInfo(symbol)
			if err != nil {
				log.Printf("Error getting ticker info for %s: %v", symbol, err)
				return
			}
			replyMessage := fmt.Sprintf("*%s*\n价格: $%.7f\n24h 涨跌: %s\n",
				info.symbol,
				info.last,
				formatChange(info.changePercent))
			replyMsg := tgbotapi.NewMessage(message.Chat.ID, replyMessage)
			replyMsg.ParseMode = "Markdown"
			bot.Send(replyMsg)
			return
		}
	}
}
