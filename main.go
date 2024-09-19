package main

import (
	"log"
	"os"
	"strconv"

	"github.com/woodchen-ink/Q58Bot/core"
	"github.com/woodchen-ink/Q58Bot/service"
	"github.com/woodchen-ink/Q58Bot/service/binance"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	botToken := os.Getenv("BOT_TOKEN")

	adminIDStr := os.Getenv("ADMIN_ID")
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Failed to get ADMIN_ID: %v", err)
	}

	err = core.Init(botToken, adminID)
	if err != nil {
		log.Fatalf("Failed to initialize service: %v", err)
	}

	err = service.RunMessageHandler()
	if err != nil {
		log.Fatalf("Error in RunMessageHandler: %v", err)
	}

	go binance.RunBinance()

	select {}
}
