package main

import (
	"log"

	"SunaiForum-Bot/core"
	"SunaiForum-Bot/service"
	"SunaiForum-Bot/service/binance"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	err := core.Init()
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}
	defer core.DB.Close() // 确保在程序退出时关闭数据库连接

	go binance.RunBinance()

	// 启动定期任务
	go service.StartScheduledTasks()

	err = service.RunMessageHandler()
	if err != nil {
		log.Fatalf("Error in RunMessageHandler: %v", err)
	}
}
