package service

import (
	"log"
	"time"

	"github.com/woodchen-ink/Q58Bot/core"
)

func StartScheduledTasks() {
	log.Printf("启动定时任务")

	go periodicCleanup()
	log.Printf("过期链接清理任务已启动")
}

func periodicCleanup() {
	ticker := time.NewTicker(24 * time.Hour) // 每天执行一次清理
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := core.DB.CleanupExpiredLinks()
			if err != nil {
				log.Printf("清理过期链接时发生错误: %v", err)
			} else {
				log.Println("已成功清理过期链接")
			}
		}
	}
}
