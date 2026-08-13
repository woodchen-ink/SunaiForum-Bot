package core

// 全局配置与启动初始化: 解析环境变量、打开数据库、建立 Bot 连接
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
	// Bot 全局唯一的 Telegram 客户端, 消息轮询与各业务模块共用
	Bot *tgbotapi.BotAPI

	BotToken    string
	ChatID      int64
	AdminID     int64
	Symbols     []string // 币安交易对, 为空表示只提供查询不主动推送
	SingaporeTZ *time.Location
	DBFile      string
	DebugMode   bool

	// AutoBanThreshold 累计违规多少次后自动封禁, 0 表示只删消息不封禁
	AutoBanThreshold int
	// DeleteServiceMessages 是否自动清理"加入/退出群组"这类群务通知
	DeleteServiceMessages bool

	DB *Database
)

// defaultAutoBanThreshold AUTO_BAN_THRESHOLD 未配置时的默认值
const defaultAutoBanThreshold = 3

// Init 按依赖顺序完成启动初始化, 任一必需项缺失都返回错误由 main 终止进程
func Init() error {
	BotToken = os.Getenv("BOT_TOKEN")
	if BotToken == "" {
		return fmt.Errorf("BOT_TOKEN 环境变量未设置")
	}

	var err error
	if AdminID, err = parseInt64(os.Getenv("ADMIN_ID")); err != nil {
		return fmt.Errorf("invalid ADMIN_ID: %w", err)
	}
	if ChatID, err = parseInt64(os.Getenv("CHAT_ID")); err != nil {
		return fmt.Errorf("invalid CHAT_ID: %w", err)
	}

	DebugMode = os.Getenv("DEBUG_MODE") == "true"
	Symbols = parseSymbols(os.Getenv("SYMBOLS"))
	AutoBanThreshold = parseIntEnv("AUTO_BAN_THRESHOLD", defaultAutoBanThreshold)
	DeleteServiceMessages = parseBoolEnv("DELETE_SERVICE_MESSAGES", true)
	SingaporeTZ = loadBusinessTZ()
	time.Local = SingaporeTZ

	DBFile = filepath.Join("/app/data", "sunai.db")
	if DB, err = NewDatabase(); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	logTableCounts("keywords", "whitelist", "prompt_replies")

	if Bot, err = tgbotapi.NewBotAPI(BotToken); err != nil {
		return fmt.Errorf("创建 Bot API 失败: %w", err)
	}
	// Debug 必须在任何模块开始用 Bot 之前设定, 之后不再写入, 避免与并发的 Send 竞争
	Bot.Debug = DebugMode
	log.Printf("[Core] 账户已授权 %s", Bot.Self.UserName)

	return nil
}

// IsAdmin 判断用户是否为机器人管理员
func IsAdmin(userID int64) bool {
	return userID == AdminID
}

// parseInt64 解析必填的数字型环境变量, 空值视为错误
func parseInt64(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}

	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("未能将 '%s' 解析为 int64: %w", s, err)
	}

	return value, nil
}

// parseIntEnv 读取可选的整数型环境变量, 缺失或非法时回退默认值并告警
func parseIntEnv(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		log.Printf("[Core] 环境变量 %s=%q 非法, 使用默认值 %d", name, raw, fallback)
		return fallback
	}
	return value
}

// parseBoolEnv 读取可选的布尔型环境变量, 缺失时用默认值; 只有明确写 false/0/no 才关闭
func parseBoolEnv(name string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "":
		return fallback
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

// parseSymbols 把 "DOGS/USDT,TON/USDT" 解析为币安接口需要的 ["DOGSUSDT","TONUSDT"];
// 未配置或全为空白时返回 nil, 调用方据此跳过价格推送
func parseSymbols(raw string) []string {
	var symbols []string
	for _, s := range strings.Split(raw, ",") {
		s = strings.ReplaceAll(strings.TrimSpace(s), "/", "")
		if s != "" {
			symbols = append(symbols, s)
		}
	}
	return symbols
}

// loadBusinessTZ 加载业务时区; 镜像缺少 tzdata 时回退到等价的固定 +8 偏移
func loadBusinessTZ() *time.Location {
	loc, err := time.LoadLocation("Asia/Singapore")
	if err != nil {
		log.Printf("[Core] 加载新加坡时区失败, 回落至固定 UTC+8: %v", err)
		return time.FixedZone("Asia/Singapore", 8*60*60)
	}
	return loc
}

// logTableCounts 启动时打印各表记录数, 便于确认数据卷是否正确挂载
func logTableCounts(tables ...string) {
	for _, table := range tables {
		count, err := DB.CountRecords(table)
		if err != nil {
			log.Printf("[Core] 检查 %s 表记录数时出错: %v", table, err)
			continue
		}
		log.Printf("[Core] %s 表中有 %d 条记录", table, count)
	}
}
