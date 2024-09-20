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
	symbolsMu sync.RWMutex
)

// LoadSymbols 初始化并缓存所有交易对
func LoadSymbols() error {
	client := binance.NewClient("", "")
	exchangeInfo, err := client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return err
	}

	symbolsMu.Lock()
	defer symbolsMu.Unlock()

	symbols = nil // 清空旧的符号列表
	for _, symbol := range exchangeInfo.Symbols {
		if symbol.Status == "TRADING" && symbol.QuoteAsset == "USDT" {
			symbols = append(symbols, symbol.BaseAsset)
		}
	}

	log.Printf("Loaded %d trading pairs", len(symbols))
	return nil
}

// GetAllSymbols 获取缓存的交易对列表
func GetAllSymbols() []string {
	symbolsMu.RLock()
	defer symbolsMu.RUnlock()
	return symbols
}

// StartSymbolRefresh 每小时刷新一次交易对缓存
func StartSymbolRefresh(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			log.Println("Refreshing trading pairs...")
			if err := LoadSymbols(); err != nil {
				log.Printf("Failed to refresh symbols: %v", err)
			}
		}
	}()
}

// HandleSymbolQuery 处理虚拟币名查询
func HandleSymbolQuery(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	// 获取所有虚拟币名
	symbols := GetAllSymbols()

	// 检查消息内容中是否包含虚拟币名
	for _, symbol := range symbols {
		if strings.Contains(strings.ToUpper(message.Text), symbol) {
			// 查询价格并回复
			info, err := getTickerInfo(symbol + "USDT") // 查询对应USDT价格
			if err != nil {
				log.Printf("Error getting ticker info for %s: %v", symbol, err)
				return
			}
			replyMessage := fmt.Sprintf("*%s*\n价格: $%.7f\n24h 涨跌: %s\n",
				info.symbol,
				info.last,
				formatChange(info.changePercent))
			msg := tgbotapi.NewMessage(message.Chat.ID, replyMessage)
			msg.ParseMode = "Markdown"
			bot.Send(msg)
			return // 找到并回复后退出
		}
	}
}
