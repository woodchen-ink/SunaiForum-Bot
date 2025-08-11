package binance

//币安价格推送
import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"SunaiForum-Bot/core"

	"github.com/adshao/go-binance/v2"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	botToken    string
	chatID      int64
	symbols     []string
	bot         *tgbotapi.BotAPI
	lastMsgID   int
	lastMsgMu   sync.Mutex // 保护lastMsgID的并发访问
)

const lastMsgIDConfigKey = "binance_last_msg_id"

// 从数据库加载lastMsgID
func loadLastMsgID() {
	lastMsgMu.Lock()
	defer lastMsgMu.Unlock()
	
	value, err := core.DB.GetConfig(lastMsgIDConfigKey)
	if err != nil {
		log.Printf("[Binance] 加载lastMsgID失败: %v", err)
		lastMsgID = 0
		return
	}
	
	if value == "" {
		lastMsgID = 0
		log.Printf("[Binance] 未找到保存的lastMsgID，设置为0")
		return
	}
	
	msgID, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("[Binance] 解析lastMsgID失败: %v", err)
		lastMsgID = 0
		return
	}
	
	lastMsgID = msgID
	log.Printf("[Binance] 从数据库加载lastMsgID: %d", lastMsgID)
}

// 保存lastMsgID到数据库
func saveLastMsgID(msgID int) {
	lastMsgMu.Lock()
	defer lastMsgMu.Unlock()
	
	lastMsgID = msgID
	err := core.DB.SetConfig(lastMsgIDConfigKey, strconv.Itoa(msgID))
	if err != nil {
		log.Printf("[Binance] 保存lastMsgID失败: %v", err)
	} else {
		log.Printf("[Binance] 保存lastMsgID到数据库: %d", msgID)
	}
}

// 获取当前的lastMsgID（线程安全）
func getLastMsgID() int {
	lastMsgMu.Lock()
	defer lastMsgMu.Unlock()
	return lastMsgID
}

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
	now := time.Now()
	message := fmt.Sprintf("市场更新 - %s (SGT)\n\n", now.Format("2006-01-02 15:04:05"))

	for _, symbol := range symbols {
		info, err := getTickerInfo(symbol)
		if err != nil {
			log.Printf("[Binance] 获取交易对 %s 的价格信息时出错: %v", symbol, err)
			continue
		}

		changeStr := formatChange(info.changePercent)

		message += fmt.Sprintf("*%s*\n", info.symbol)
		message += fmt.Sprintf("价格: $%.7f\n", info.last)
		message += fmt.Sprintf("24h 涨跌: %s\n\n", changeStr)
	}

	// 删除之前的消息（如果存在）
	currentLastMsgID := getLastMsgID()
	if currentLastMsgID != 0 {
		deleteMsg := tgbotapi.NewDeleteMessage(chatID, currentLastMsgID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			log.Printf("[Binance] 删除前一条消息 %d 时出错: %v", currentLastMsgID, err)
		} else {
			log.Printf("[Binance] 成功删除前一条消息: %d", currentLastMsgID)
		}
	}

	// 发送新消息
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	sentMsg, err := bot.Send(msg)
	if err != nil {
		log.Printf("[Binance] 发送消息时出错: %v\nFull message content:\nChat ID: %d\nMessage: %s", err, chatID, message)
		return
	}

	// 保存新消息ID到数据库
	saveLastMsgID(sentMsg.MessageID)
}

func RunBinance() {
	log.Println("[Binance]", "启动币安服务...")

	// 初始化必要的变量
	botToken = core.BOT_TOKEN
	bot = core.Bot
	chatID = core.ChatID
	symbols = core.Symbols

	// 从数据库加载lastMsgID（容器重启时恢复）
	loadLastMsgID()

	// 初始化并加载所有交易对
	if err := LoadAllSymbols(); err != nil {
		log.Fatalf("[Binance] 加载所有交易对失败: %v", err)
	}

	// 启动每小时刷新交易对缓存
	go StartSymbolRefresh(1 * time.Hour)
	log.Println("[Binance]", "启动每小时刷新交易对缓存...")

	// 立即发送一次价格更新（会删除之前的消息如果存在）
	sendPriceUpdate()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		if now.Minute() == 0 {
			log.Println("[Binance]", "发送每小时价格更新...")
			sendPriceUpdate()
		}
	}
}
