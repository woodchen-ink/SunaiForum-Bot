package core

var (
	BOT_TOKEN string
	ADMIN_ID  int64
)

func InitGlobalVariables(botToken string, adminID int64) {
	BOT_TOKEN = botToken
	ADMIN_ID = adminID
}

func IsAdmin(userID int64) bool {
	return userID == ADMIN_ID
}
