package main

import (
	"log"
	"os"
	"strconv"

	"github.com/woodchen-ink/Q58Bot/service"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	botToken := os.Getenv("BOT_TOKEN")
	adminIDStr := os.Getenv("ADMIN_ID")
	adminID, err := strconv.ParseInt(adminIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid ADMIN_ID: %v", err)
	}

	service.Init(botToken, adminID)

	go service.RunGuard()
	go service.RunBinance()

	select {}
}
