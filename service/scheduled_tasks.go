package service

// 后台定时任务
import (
	"log"
	"time"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service/ai_review"
	"SunaiForum-Bot/service/moderation"
)

const (
	// cleanupInterval 数据清理任务的执行间隔
	cleanupInterval = 24 * time.Hour
	// repeatHistoryTTL 刷屏记录中用户多久不活跃即清除
	repeatHistoryTTL = time.Hour
	// strikeTTL 违规计分多久无新增即归零, 相当于给用户的自动改过窗口
	strikeTTL = 30 * 24 * time.Hour
	// userStatsTTL 发言统计多久不活跃即清除; 清除后该用户再发言会重新按新用户走 AI 审核
	userStatsTTL = 90 * 24 * time.Hour
	// actionTTL 处置记录保留时长, 过期后对应的撤销按钮失效
	actionTTL = 30 * 24 * time.Hour
)

// StartScheduledTasks 拉起全部后台定时任务, 立即返回
func StartScheduledTasks() {
	log.Println("[Scheduler] 启动定时任务")
	go periodicCleanup()
	ai_review.StartCuration(core.Bot)
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
	cleanupRows("陈旧发言统计", func() (int64, error) { return core.DB.CleanupStaleUserStats(userStatsTTL) })
	cleanupRows("过期处置记录", func() (int64, error) { return core.DB.CleanupOldActions(actionTTL) })

	if removed := moderation.PruneRepeatHistory(repeatHistoryTTL); removed > 0 {
		log.Printf("[Scheduler] 已清理 %d 个不活跃用户的刷屏记录", removed)
	}
}

// cleanupRows 跑一个删除类清理并统一记日志
func cleanupRows(what string, clean func() (int64, error)) {
	rowsAffected, err := clean()
	if err != nil {
		log.Printf("[Scheduler] 清理%s失败: %v", what, err)
		return
	}
	if rowsAffected > 0 {
		log.Printf("[Scheduler] 已清理 %d 条%s", rowsAffected, what)
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
