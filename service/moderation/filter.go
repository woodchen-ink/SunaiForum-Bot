package moderation

// 群消息内容审核入口: 命中过滤关键词的消息直接撤回。
// 关键词每次从 core.DB 的 TTL 缓存读取, 管理员 /add /delete 后立即生效, 无需重启。
import (
	"log"
	"strings"
	"time"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// logTextLimit 日志里截断消息文本的字符数, 只影响日志可读性, 不影响匹配范围
const logTextLimit = 120

// ShouldFilter 判断文本是否命中过滤关键词, 返回命中的关键词。
// 查库失败时按"不过滤"处理, 宁可漏拦也不误删用户消息。
func ShouldFilter(text string) (bool, string) {
	if strings.TrimSpace(text) == "" {
		return false, ""
	}

	keywords, err := core.DB.GetAllManualKeywords()
	if err != nil {
		log.Printf("[Moderation] 读取关键词失败, 本条消息放行: %v", err)
		return false, ""
	}

	lowered := strings.ToLower(text)
	for _, keyword := range keywords {
		if keyword != "" && strings.Contains(lowered, strings.ToLower(keyword)) {
			return true, keyword
		}
	}
	return false, ""
}

// CheckAndFilter 命中关键词则撤回消息并发送 3 分钟后自毁的提示, 返回是否已拦截该消息
func CheckAndFilter(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	hit, keyword := ShouldFilter(message.Text)
	if !hit {
		return false
	}

	log.Printf("[Moderation] 命中关键词 %q, 撤回消息: %s", keyword, truncate(message.Text, logTextLimit))

	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	if _, err := bot.Request(deleteMsg); err != nil {
		log.Printf("[Moderation] 删除消息失败: %v", err)
		return true
	}

	notification := tgbotapi.NewMessage(message.Chat.ID, "已撤回该消息。")
	sent, err := bot.Send(notification)
	if err != nil {
		log.Printf("[Moderation] 发送提示失败: %v", err)
		return true
	}

	core.DeleteMessageAfterDelay(bot, message.Chat.ID, sent.MessageID, 3*time.Minute)
	return true
}

// truncate 按字符数截断字符串用于日志输出, 不会切坏多字节字符
func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
