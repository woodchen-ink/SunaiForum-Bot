package group_member_management

// 群成员管理: 管理员回复某条消息发 /ban 即可删消息并永久封禁其作者
import (
	"fmt"
	"log"
	"time"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// noticeTTL 机器人提示与管理员指令的自毁时间, 避免群里堆积管理痕迹
const noticeTTL = 3 * time.Minute

// HandleBanCommand 处理管理员对某条消息的 /ban 回复:
// 删除被回复的原消息、永久踢出其作者, 并在群内留一条限时提示
func HandleBanCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	if message.From == nil || !core.IsAdmin(message.From.ID) {
		return
	}
	// 被回复的消息可能来自匿名管理员或频道身份, 此时拿不到可封禁的用户
	if message.ReplyToMessage == nil || message.ReplyToMessage.From == nil {
		return
	}

	chatID := message.Chat.ID
	userToBan := message.ReplyToMessage.From

	core.DeleteMessages(bot, chatID, message.ReplyToMessage.MessageID)

	kickConfig := tgbotapi.KickChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatID,
			UserID: userToBan.ID,
		},
		UntilDate: 0, // 0 表示永久封禁
	}
	if _, err := bot.Request(kickConfig); err != nil {
		log.Printf("[GroupMemberManagement] 封禁用户 %d 失败: %v", userToBan.ID, err)
		return
	}
	log.Printf("[GroupMemberManagement] 已封禁用户 %s (ID: %d)", userToBan.UserName, userToBan.ID)

	notice := tgbotapi.NewMessage(chatID, fmt.Sprintf("用户 %s 已被封禁并踢出群组。", userToBan.UserName))
	sentMsg, err := bot.Send(notice)
	if err != nil {
		log.Printf("[GroupMemberManagement] 发送封禁提示失败: %v", err)
		return
	}

	core.DeleteMessageAfterDelay(bot, chatID, sentMsg.MessageID, noticeTTL)
	core.DeleteMessageAfterDelay(bot, chatID, message.MessageID, noticeTTL)
}
