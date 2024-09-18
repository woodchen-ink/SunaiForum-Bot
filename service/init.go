package service

import (
	"time"

	"github.com/woodchen-ink/Q58Bot/core"
)

func Init(botToken string, adminID int64) {
	core.InitGlobalVariables(botToken, adminID)

	// 设置时区
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc
}
