package service

import (
	"time"

	"github.com/woodchen-ink/Q58Bot/core"
)

func Init(botToken string, adminID int64) error {
	core.InitGlobalVariables(botToken, adminID)

	// 初始化提示词服务
	err := InitPromptService()
	if err != nil {
		return err
	}

	// 设置时区
	loc := time.FixedZone("Asia/Singapore", 8*60*60)
	time.Local = loc

	return nil
}
