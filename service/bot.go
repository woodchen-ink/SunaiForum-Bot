package service

// 机器人生命周期: 注册命令、拉取更新、断线重连
import (
	"fmt"
	"log"
	"time"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/prompt_reply"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// updateTimeout long polling 单次等待秒数
	updateTimeout = 60
	// reconnectBaseDelay / reconnectMaxDelay 断线重连的指数退避区间
	reconnectBaseDelay = time.Second
	reconnectMaxDelay  = 5 * time.Minute
	// healthyRunThreshold 连接维持超过这个时长即视为健康, 断开后退避重新计起
	healthyRunThreshold = time.Minute
)

// RunMessageHandler 阻塞运行消息处理主循环, 仅在无法恢复的初始化错误时返回
func RunMessageHandler() error {
	log.Println("[MessageHandler] 消息处理器启动...")

	// 自动回复加载失败不阻断启动: 其它过滤功能仍应工作
	if err := prompt_reply.Manager.LoadDataFromDatabase(); err != nil {
		log.Printf("[MessageHandler] 加载提示回复数据失败, 自动回复暂不可用: %v", err)
	}

	bot := core.Bot
	if err := core.RegisterCommands(bot); err != nil {
		return fmt.Errorf("注册机器人命令失败: %w", err)
	}
	log.Printf("[MessageHandler] 已授权账户 %s", bot.Self.UserName)

	rateLimiter := core.NewRateLimiter()
	delay := reconnectBaseDelay

	for {
		startedAt := time.Now()
		consumeUpdates(bot, rateLimiter)

		// 正常跑过一段时间才断开的, 视为偶发断线, 退避重新从最小值开始
		if time.Since(startedAt) > healthyRunThreshold {
			delay = reconnectBaseDelay
		}

		log.Printf("[MessageHandler] 更新通道已关闭, %v 后重连...", delay)
		time.Sleep(delay)

		delay *= 2
		if delay > reconnectMaxDelay {
			delay = reconnectMaxDelay
		}
	}
}

// consumeUpdates 建立更新通道并分发消息, 通道关闭后返回交由上层重连
func consumeUpdates(bot *tgbotapi.BotAPI, rateLimiter *core.RateLimiter) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = updateTimeout

	for update := range bot.GetUpdatesChan(u) {
		go func() {
			// 单条消息的处理失败不允许拖垮整个进程
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[MessageHandler] 处理更新 %d 时 panic: %v", update.UpdateID, r)
				}
			}()
			handleUpdate(bot, update, rateLimiter)
		}()
	}
}
