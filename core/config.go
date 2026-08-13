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

	BotToken   string
	ChatID     int64
	AdminID    int64
	Symbols    []string // 币安交易对, 为空表示只提供查询不主动推送
	BusinessTZ *time.Location
	DBFile     string
	DebugMode  bool

	// AutoBanThreshold 累计违规多少次后自动封禁, 0 表示只删消息不封禁
	AutoBanThreshold int
	// DeleteServiceMessages 是否自动清理"加入/退出群组"这类群务通知
	DeleteServiceMessages bool

	// AI 审核配置; AIAPIKey 为空时整个 AI 层关闭, 只跑确定性规则
	AIEnabled          bool
	AIBaseURL          string
	AIAPIKey           string
	AIModel            string
	AIReasoningEffort  string
	AINewUserMessages  int // 新用户前多少条消息全量送 AI 审核
	AIHourlyBudget     int // 全局每小时最大调用次数, 防止异常情况下跑量
	AIMinConfidence    float64
	AICurationInterval time.Duration // AI 整理词表的间隔

	DB *Database
)

const (
	defaultAutoBanThreshold = 3
	defaultAIBaseURL        = "https://ai.czl.net/v1"
	defaultAIModel          = "gpt-5.6-luna"
	defaultAIReasoning      = "high"
	defaultAINewUserMsgs    = 3
	defaultAIHourlyBudget   = 200
	defaultAIMinConfidence  = 0.8
	defaultCurationInterval = 7 * 24 * time.Hour
	defaultTimezone         = "Asia/Shanghai"
)

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
	initAIConfig()
	BusinessTZ = loadBusinessTZ(envOr("TZ", defaultTimezone))
	time.Local = BusinessTZ

	DBFile = filepath.Join("/app/data", "sunai.db")
	if DB, err = NewDatabase(); err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	logTableCounts()

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

// parseInt64 解析必填的数字型环境变量, 空值视为错误。
// 面板里粘贴的值常带尾随空格, 不清理会让机器人直接起不来。
func parseInt64(s string) (int64, error) {
	s = cleanEnvValue(s)
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}

	value, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("未能将 '%s' 解析为 int64: %w", s, err)
	}

	return value, nil
}

// cleanEnvValue 清理数值/布尔型环境变量的值: 剥掉行内 "#" 注释再去首尾空白。
//
// Docker 的 env 格式里 "#" 之后**不算注释**, 但面板上写 "AUTO_BAN_THRESHOLD=0  # 观察期"
// 是很自然的习惯; 不处理的话值会变成 "0  # 观察期", 解析失败静默回落默认值 3,
// 结果是"以为关了自动封禁, 其实开着"。
//
// 只对数值和布尔用: 字符串型的值 (API key、token) 可能合法包含 "#", 不能这样切。
func cleanEnvValue(s string) string {
	if idx := strings.IndexByte(s, '#'); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// initAIConfig 读取 AI 审核相关配置。
// AI_API_KEY 缺失即整层关闭 —— 没有密钥时静默降级到确定性规则, 而不是每条消息都报错。
func initAIConfig() {
	AIAPIKey = os.Getenv("AI_API_KEY")
	AIBaseURL = strings.TrimSuffix(envOr("AI_BASE_URL", defaultAIBaseURL), "/")
	AIModel = envOr("AI_MODEL", defaultAIModel)
	AIReasoningEffort = envOr("AI_REASONING_EFFORT", defaultAIReasoning)
	AINewUserMessages = parseIntEnv("AI_NEW_USER_MESSAGES", defaultAINewUserMsgs)
	AIHourlyBudget = parseIntEnv("AI_HOURLY_BUDGET", defaultAIHourlyBudget)
	AIMinConfidence = parseFloatEnv("AI_MIN_CONFIDENCE", defaultAIMinConfidence)
	AICurationInterval = defaultCurationInterval

	AIEnabled = AIAPIKey != "" && parseBoolEnv("AI_ENABLED", true)
	if AIEnabled {
		log.Printf("[Core] AI 审核已启用: model=%s effort=%s 新用户前 %d 条 每小时上限 %d 次",
			AIModel, AIReasoningEffort, AINewUserMessages, AIHourlyBudget)
	} else {
		log.Println("[Core] AI 审核未启用 (AI_API_KEY 未设置), 只运行确定性规则")
	}
}

// envOr 读取环境变量, 为空时返回默认值
func envOr(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

// parseFloatEnv 读取可选的浮点型环境变量, 缺失或非法时回退默认值
func parseFloatEnv(name string, fallback float64) float64 {
	raw := cleanEnvValue(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || value > 1 {
		log.Printf("[Core] 环境变量 %s=%q 非法, 使用默认值 %v", name, raw, fallback)
		return fallback
	}
	return value
}

// parseIntEnv 读取可选的整数型环境变量, 缺失或非法时回退默认值并告警
func parseIntEnv(name string, fallback int) int {
	raw := cleanEnvValue(os.Getenv(name))
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
	switch strings.ToLower(cleanEnvValue(os.Getenv(name))) {
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

// loadBusinessTZ 按名称加载业务时区; 镜像缺 tzdata 或名称非法时回退到固定 +8 偏移。
// 不静默回落到 UTC —— 那会让所有时间显示悄悄错 8 小时。
func loadBusinessTZ(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		log.Printf("[Core] 加载时区 %q 失败, 回落至固定 UTC+8: %v", name, err)
		return time.FixedZone("UTC+8", 8*60*60)
	}
	log.Printf("[Core] 业务时区: %s", name)
	return loc
}

// logTableCounts 启动时打印各表记录数, 便于确认数据卷是否正确挂载
func logTableCounts() {
	tables := []struct {
		name  string
		model any
	}{
		{"keywords", &Keyword{}},
		{"prompt_replies", &PromptReply{}},
	}

	for _, t := range tables {
		count, err := DB.CountRecords(t.model)
		if err != nil {
			log.Printf("[Core] 检查 %s 表记录数时出错: %v", t.name, err)
			continue
		}
		log.Printf("[Core] %s 表中有 %d 条记录", t.name, count)
	}
}
