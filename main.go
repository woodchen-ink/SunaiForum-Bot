package main

import (
	"log"

	"github.com/woodchen-ink/Q58Bot/core"
	"github.com/woodchen-ink/Q58Bot/service"
	"github.com/woodchen-ink/Q58Bot/service/binance"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	err := core.Init()
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}
	defer core.DB.Close() // 确保在程序退出时关闭数据库连接

	go binance.RunBinance()

	err = service.RunMessageHandler()
	if err != nil {
		log.Fatalf("Error in RunMessageHandler: %v", err)
	}

	// 启动定期任务
	service.StartScheduledTasks()
}
