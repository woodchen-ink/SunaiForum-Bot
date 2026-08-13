package command

// 待输入状态。
//
// 存在的原因: 点 Telegram 菜单只会发出一个裸命令 (/add), 带不了参数。
// 机器人记住"这个管理员正在为 /add 补参数", 把下一条普通消息当作参数消费。
//
// 只放内存: 状态生命周期以分钟计, 重启后管理员重发一次命令即可, 不值得落库。
import (
	"sync"
	"time"
)

// pendingTTL 待输入状态的存活时间; 超时后下一条消息不再被当成参数
const pendingTTL = 5 * time.Minute

type pendingCommand struct {
	name     string
	expireAt time.Time
}

var pending = struct {
	mu    sync.Mutex
	items map[int64]pendingCommand
}{items: make(map[int64]pendingCommand)}

// setPending 记下某管理员正在为哪个命令补参数
func setPending(userID int64, name string) {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	pending.items[userID] = pendingCommand{name: name, expireAt: time.Now().Add(pendingTTL)}
}

// takePending 取出并清除待输入的命令; 已过期视为没有
func takePending(userID int64) (string, bool) {
	pending.mu.Lock()
	defer pending.mu.Unlock()

	item, ok := pending.items[userID]
	if !ok {
		return "", false
	}
	delete(pending.items, userID)

	if time.Now().After(item.expireAt) {
		return "", false
	}
	return item.name, true
}

// clearPending 丢弃待输入状态, 返回此前是否真的有一条在等
func clearPending(userID int64) bool {
	pending.mu.Lock()
	defer pending.mu.Unlock()

	_, ok := pending.items[userID]
	delete(pending.items, userID)
	return ok
}
