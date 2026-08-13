package service

// 后台定时任务
import (
	"log"
	"time"

	"SunaiForum-Bot/core"
)

// cleanupInterval 数据清理任务的执行间隔
const cleanupInterval = 24 * time.Hour

// StartScheduledTasks 拉起全部后台定时任务, 立即返回
func StartScheduledTasks() {
	log.Println("[Scheduler] 启动定时任务")
	go periodicCleanup()
}

// periodicCleanup 每天清理一次历史遗留数据, 启动时先跑一轮
func periodicCleanup() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	cleanupLegacyAutoLinks()
	for range ticker.C {
		cleanupLegacyAutoLinks()
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
