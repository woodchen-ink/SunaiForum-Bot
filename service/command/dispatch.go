package command

// 管理员命令的注册表与分发。
//
// 一张表同时驱动三件事: Telegram 菜单注册、命令分发、缺参数时的追问。
// 三者共用一份定义, 就不会出现"菜单里有但代码没实现"或"改了处理逻辑忘了改菜单"的漂移。
import (
	"fmt"
	"sort"
	"strings"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// spec 一个管理员命令的完整定义
type spec struct {
	desc      string // Telegram 菜单里的说明
	order     int    // 菜单里的排列顺序
	needsArgs bool   // 是否需要参数; 为 true 且用户没带参数时会追问
	askFor    string // 追问的提示语
	handle    func(bot *tgbotapi.BotAPI, message *tgbotapi.Message, args string)
}

// specs 全部管理员命令。新增命令只需要在这里加一行。
var specs = map[string]spec{
	"add": {
		desc: "添加过滤关键词", order: 1, needsArgs: true,
		askFor: "请发送要添加的关键词。\n可以一次发多个，每行一个。\n\n发送 /cancel 取消。",
		handle: addKeywords,
	},
	"delete": {
		desc: "删除过滤关键词", order: 2, needsArgs: true,
		askFor: "请发送要删除的关键词。\n可以一次发多个，每行一个。\n\n发送 /cancel 取消。",
		handle: deleteKeywords,
	},
	"deletecontaining": {
		desc: "删除所有包含指定词语的关键词", order: 3, needsArgs: true,
		askFor: "请发送要匹配的词语，所有包含它的关键词都会被删除。\n\n发送 /cancel 取消。",
		handle: deleteKeywordsContaining,
	},
	"list": {
		desc: "列出所有关键词", order: 4,
		handle: func(bot *tgbotapi.BotAPI, message *tgbotapi.Message, _ string) { listKeywords(bot, message) },
	},
	"setprompt": {
		desc: "设置自动回复", order: 5, needsArgs: true,
		askFor: "请发送触发词和回复内容。\n第一行是触发词，之后所有行是回复内容。\n\n发送 /cancel 取消。",
		handle: setPrompt,
	},
	"delprompt": {
		desc: "删除自动回复", order: 6, needsArgs: true,
		askFor: "请发送要删除的触发词。\n\n发送 /cancel 取消。",
		handle: deletePrompt,
	},
	"listprompt": {
		desc: "列出所有自动回复", order: 7,
		handle: func(bot *tgbotapi.BotAPI, message *tgbotapi.Message, _ string) { listPrompts(bot, message) },
	},
	"cancel": {
		desc: "取消当前正在输入的命令", order: 8,
		handle: cancelPending,
	},
}

// MenuCommands 按定义表生成 Telegram 命令菜单
func MenuCommands() []tgbotapi.BotCommand {
	names := make([]string, 0, len(specs))
	for name := range specs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return specs[names[i]].order < specs[names[j]].order })

	commands := make([]tgbotapi.BotCommand, 0, len(names))
	for _, name := range names {
		commands = append(commands, tgbotapi.BotCommand{Command: name, Description: specs[name].desc})
	}
	return commands
}

// HandleAdmin 处理管理员私聊消息。
//
// 两种进入方式:
//   - 一步式: /add 关键词
//   - 两步式: 先发 /add (比如从菜单点击), 机器人追问, 再发关键词
//
// 第二种是为了菜单可用 —— 点菜单只会发出裸命令, 没有参数。
func HandleAdmin(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	name := message.Command()

	// 不是命令: 可能是在补充上一条命令的参数
	if name == "" {
		if pendingCmd, ok := takePending(message.From.ID); ok {
			runCommand(bot, message, pendingCmd, message.Text)
			return
		}
		core.SendMessage(bot, message.Chat.ID, "不知道你想做什么，点击菜单或发送 /list 看看。")
		return
	}

	// 发新命令时丢弃上一条未完成的输入, 避免参数串到别的命令上
	if name != "cancel" {
		clearPending(message.From.ID)
	}

	cmd, ok := specs[name]
	if !ok {
		core.SendErrorMessage(bot, message.Chat.ID, "未知命令，点击菜单看看有哪些可用。")
		return
	}

	args := strings.TrimSpace(message.CommandArguments())
	if cmd.needsArgs && args == "" {
		setPending(message.From.ID, name)
		ask(bot, message.Chat.ID, cmd.askFor)
		return
	}

	runCommand(bot, message, name, args)
}

// runCommand 执行命令; 命令名来自 specs, 调用前已确认存在
func runCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, name, args string) {
	cmd, ok := specs[name]
	if !ok {
		core.SendErrorMessage(bot, message.Chat.ID, "未知命令。")
		return
	}

	args = strings.TrimSpace(args)
	if cmd.needsArgs && args == "" {
		core.SendErrorMessage(bot, message.Chat.ID, "内容不能为空，请重新发送命令。")
		return
	}
	cmd.handle(bot, message, args)
}

// ask 发出追问, 并让客户端自动聚焦输入框
func ask(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: "在这里输入…"}
	if _, err := bot.Send(msg); err != nil {
		core.SendMessage(bot, chatID, text)
	}
}

// cancelPending 取消正在等待输入的命令
func cancelPending(bot *tgbotapi.BotAPI, message *tgbotapi.Message, _ string) {
	if clearPending(message.From.ID) {
		core.SendMessage(bot, message.Chat.ID, "已取消。")
		return
	}
	core.SendMessage(bot, message.Chat.ID, "当前没有待输入的命令。")
}

// splitLines 把多行输入拆成逐条参数, 去空行和首尾空白。
// 支持批量是因为管理员补词时通常是一次贴一批, 逐条发命令太累。
func splitLines(input string) []string {
	var items []string
	for _, line := range strings.Split(input, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			items = append(items, line)
		}
	}
	return items
}

// batchReport 汇总批量操作的结果
type batchReport struct {
	succeeded []string
	skipped   []string // 已存在 / 未找到这类非错误情况
	failed    []string // 带原因
}

func (r *batchReport) render(succeededLabel, skippedLabel string) string {
	var b strings.Builder

	if len(r.succeeded) > 0 {
		fmt.Fprintf(&b, "%s（%d）：\n%s\n", succeededLabel, len(r.succeeded), strings.Join(r.succeeded, "、"))
	}
	if len(r.skipped) > 0 {
		fmt.Fprintf(&b, "\n%s（%d）：\n%s\n", skippedLabel, len(r.skipped), strings.Join(r.skipped, "、"))
	}
	if len(r.failed) > 0 {
		fmt.Fprintf(&b, "\n失败（%d）：\n%s\n", len(r.failed), strings.Join(r.failed, "\n"))
	}
	if b.Len() == 0 {
		return "没有任何内容被处理。"
	}
	return strings.TrimSpace(b.String())
}
