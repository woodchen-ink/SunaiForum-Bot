package core

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	Bot         *tgbotapi.BotAPI
	BOT_TOKEN   string
	ChatID      int64
	ADMIN_ID    int64
	Symbols     []string
	SingaporeTZ *time.Location
	DB_FILE     string
	DEBUG_MODE  bool
	err         error
)

func IsAdmin(userID int64) bool {
	return userID == ADMIN_ID
}
func mustParseInt64(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}

	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("未能将'%s'解析为 int64: %v", s, err)
	}

	return value, nil
}
func Init(botToken string, adminID int64) error {

	// 设置数据库文件路径
	DB_FILE = filepath.Join("/app/data", "q58.db")

	// 从环境变量中读取调试模式设置
	DEBUG_MODE = os.Getenv("DEBUG_MODE") == "true"

	// 设置时区
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc

	// 初始化 Chat ID
	chatIDStr := os.Getenv("CHAT_ID")
	ChatID, err = mustParseInt64(chatIDStr)
	if err != nil {
		log.Fatalf("Invalid CHAT_ID: %v", err)
	}

	// 初始化 Symbols
	symbolsRaw := strings.Split(os.Getenv("SYMBOLS"), ",")
	Symbols = make([]string, len(symbolsRaw))
	for i, s := range symbolsRaw {
		Symbols[i] = strings.ReplaceAll(s, "/", "")
	}

	// 初始化新加坡时区
	SingaporeTZ, err = time.LoadLocation("Asia/Singapore")
	if err != nil {
		log.Printf("加载新加坡时区时出错: %v", err)
		log.Println("回落至 UTC+8")
		SingaporeTZ = time.FixedZone("Asia/Singapore", 8*60*60)
	}

	// 初始化 Bot API
	Bot, err = tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("账户已授权 %s", Bot.Self.UserName)
	return nil
}
