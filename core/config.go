package core

import (
	"os"
	"path/filepath"
)

var (
	BOT_TOKEN  string
	ADMIN_ID   int64
	DB_FILE    string
	DEBUG_MODE bool
)

func InitGlobalVariables(botToken string, adminID int64) {
	BOT_TOKEN = botToken
	ADMIN_ID = adminID

	// 设置数据库文件路径
	DB_FILE = filepath.Join("/app/data", "q58.db")

	// 从环境变量中读取调试模式设置
	DEBUG_MODE = os.Getenv("DEBUG_MODE") == "true"
}

func IsAdmin(userID int64) bool {
	return userID == ADMIN_ID
}
