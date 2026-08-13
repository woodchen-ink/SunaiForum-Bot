package service

// 后台定时任务
import (
	"log"
	"time"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/moderation"
)

const (
	// cleanupInterval 数据清理任务的执行间隔
	cleanupInterval = 24 * time.Hour
	// repeatHistoryTTL 刷屏记录中用户多久不活跃即清除
	repeatHistoryTTL = time.Hour
	// strikeTTL 违规计分多久无新增即归零, 相当于给用户的自动改过窗口
	strikeTTL = 30 * 24 * time.Hour
)

// StartScheduledTasks 拉起全部后台定时任务, 立即返回
func StartScheduledTasks() {
	log.Println("[Scheduler] 启动定时任务")
	go periodicCleanup()
}

// periodicCleanup 每天清理一次历史遗留数据, 启动时先跑一轮
func periodicCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	runCleanup()
	for range ticker.C {
		runCleanup()
	}
}

// runCleanup 跑一轮全部清理动作
func runCleanup() {
	cleanupLegacyAutoLinks()
	cleanupStaleStrikes()

	if removed := moderation.PruneRepeatHistory(repeatHistoryTTL); removed > 0 {
		log.Printf("[Scheduler] 已清理 %d 个不活跃用户的刷屏记录", removed)
	}
}

// cleanupStaleStrikes 清掉长期无新违规的计分, 相当于给改过自新的用户一个自动重置
func cleanupStaleStrikes() {
	rowsAffected, err := core.DB.CleanupStaleStrikes(strikeTTL)
	if err != nil {
		log.Printf("[Scheduler] 清理陈旧违规计分失败: %v", err)
		return
	}
	if rowsAffected > 0 {
		log.Printf("[Scheduler] 已清理 %d 条陈旧违规计分", rowsAffected)
	}
}

// cleanupLegacyAutoLinks 排干"同一链接不能发两次"功能下线后残留的自动关键词
func cleanupLegacyAutoLinks() {
	rowsAffected, err := core.DB.CleanupLegacyAutoLinks()
	if err != nil {
		log.Printf("[Scheduler] 清理历史自动关键词失败: %v", err)
		return
	}
	if rowsAffected > 0 {
		log.Printf("[Scheduler] 已清理 %d 条历史自动关键词", rowsAffected)
	}
}
