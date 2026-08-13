package group_member_management

// 群务通知清理: 自动删除 "X 加入群组" "X 退出群组" 这类 Telegram 自动生成的系统消息。
// 这些消息是纯噪音, 尤其在有人批量拉小号进群时会把正常聊天顶掉。
import (
	"log"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// IsServiceMessage 判断是否为需要清理的群务通知。
//
// 不含"置顶了一条消息": 置顶是管理员的主动动作, 那条通知本身有意义, 删掉反而让人不知道发生了什么。
func IsServiceMessage(message *tgbotapi.Message) bool {
	switch {
	case len(message.NewChatMembers) > 0: // 有人加入
		return true
	case message.LeftChatMember != nil: // 有人退出或被移出
		return true
	case message.NewChatTitle != "": // 群名变更
		return true
	case len(message.NewChatPhoto) > 0, message.DeleteChatPhoto: // 群头像变更
		return true
	default:
		return false
	}
}

// CleanServiceMessage 删除群务通知, 返回是否处理了本条消息。
// 由 DeleteServiceMessages 开关控制; 机器人没有删除权限时只记日志, 不中断后续流程。
func CleanServiceMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	if !IsServiceMessage(message) {
		return false
	}
	if !core.DeleteServiceMessages {
		return true // 仍然算作已处理: 群务通知没有正文, 不需要走后续的内容审核
	}

	deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	if _, err := bot.Request(deleteMsg); err != nil {
		log.Printf("[GroupMemberManagement] 删除群务通知失败 (可能缺少删除消息权限): %v", err)
	}
	return true
}
