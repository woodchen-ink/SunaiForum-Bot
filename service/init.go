package service

import (
	"os"
	"time"

	"github.com/woodchen-ink/Q58Bot/core"
)

var (
	dbFile    string
	debugMode bool
)

func Init(botToken string, adminID int64) {
	core.InitGlobalVariables(botToken, adminID)
	dbFile = "/app/data/q58.db"
	debugMode = os.Getenv("DEBUG_MODE") == "true"

	// 设置时区
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc
}
