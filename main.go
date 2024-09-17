package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/woodchen-ink/Q58Bot/service"
)

var (
	BOT_TOKEN string
	ADMIN_ID  int64
)

func init() {
	// 设置时区
	setTimeZone()

	// 其他初始化逻辑
	initializeVariables()
}

func setTimeZone() {
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc
}

func initializeVariables() {
	BOT_TOKEN = os.Getenv("BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID")
	var err error
	ADMIN_ID, err = strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid ADMIN_ID: %v", err)
	}
}

func runGuard() {
	for {
		try(func() {
			service.RunGuard()
		}, "Guard")
	}
}

func runBinance() {
	for {
		try(func() {
			service.RunBinance()
		}, "Binance")
	}
}

func try(fn func(), name string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("%s process crashed: %v", name, r)
			log.Printf("Restarting %s process...", name)
			time.Sleep(time.Second) // 添加短暂延迟以防止过快重启
		}
	}()
	fn()
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 使用 goroutines 运行 guard 和 binance 服务
	go runGuard()
	go runBinance()

	// 保持主程序运行
	select {}
}
