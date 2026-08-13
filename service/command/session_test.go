package command

import (
	"testing"
	"time"
)

// TestPendingLifecycle 待输入状态的取用与清除
func TestPendingLifecycle(t *testing.T) {
	const userID = int64(7001)
	t.Cleanup(func() { clearPending(userID) })

	if _, ok := takePending(userID); ok {
		t.Fatal("初始状态不该有待输入命令")
	}

	setPending(userID, "add")
	name, ok := takePending(userID)
	if !ok || name != "add" {
		t.Errorf("取出待输入命令 = %q,%v, 期望 add,true", name, ok)
	}

	// 取出即消费, 同一条参数不该被两个命令重复使用
	if _, ok := takePending(userID); ok {
		t.Error("待输入命令取出后应当已被清除")
	}
}

// TestPendingExpires 超时后的待输入状态不再被当作参数消费
func TestPendingExpires(t *testing.T) {
	const userID = int64(7002)
	t.Cleanup(func() { clearPending(userID) })

	pending.mu.Lock()
	pending.items[userID] = pendingCommand{name: "add", expireAt: time.Now().Add(-time.Second)}
	pending.mu.Unlock()

	if _, ok := takePending(userID); ok {
		t.Error("已过期的待输入命令不该被消费")
	}
}

// TestPendingIsolatedPerUser 不同用户的待输入状态互不干扰
func TestPendingIsolatedPerUser(t *testing.T) {
	const userA, userB = int64(7003), int64(7004)
	t.Cleanup(func() { clearPending(userA); clearPending(userB) })

	setPending(userA, "add")
	setPending(userB, "delete")

	if name, _ := takePending(userA); name != "add" {
		t.Errorf("用户 A 的待输入命令 = %q, 期望 add", name)
	}
	if name, _ := takePending(userB); name != "delete" {
		t.Errorf("用户 B 的待输入命令 = %q, 期望 delete", name)
	}
}

// TestClearPendingReportsPresence clearPending 要如实反映此前是否有待输入
func TestClearPendingReportsPresence(t *testing.T) {
	const userID = int64(7005)
	t.Cleanup(func() { clearPending(userID) })

	if clearPending(userID) {
		t.Error("没有待输入时应当返回 false")
	}
	setPending(userID, "add")
	if !clearPending(userID) {
		t.Error("有待输入时应当返回 true")
	}
}

func TestSplitLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"水果机", []string{"水果机"}},
		{"水果机\n日入1w\n代收款", []string{"水果机", "日入1w", "代收款"}},
		{"  水果机  \n\n\n  日入1w  ", []string{"水果机", "日入1w"}},
		{"\n\n", nil},
		{"", nil},
	}

	for _, c := range cases {
		got := splitLines(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitLines(%q) = %v, 期望 %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitLines(%q)[%d] = %q, 期望 %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// TestEveryMenuCommandHasHandler 菜单里出现的命令必须都有实现, 不能点了没反应
func TestEveryMenuCommandHasHandler(t *testing.T) {
	for _, cmd := range MenuCommands() {
		spec, ok := specs[cmd.Command]
		if !ok {
			t.Errorf("菜单命令 /%s 在定义表里找不到", cmd.Command)
			continue
		}
		if spec.handle == nil {
			t.Errorf("命令 /%s 没有处理函数", cmd.Command)
		}
		if spec.needsArgs && spec.askFor == "" {
			t.Errorf("命令 /%s 需要参数但没写追问提示, 点菜单会卡住", cmd.Command)
		}
		if cmd.Description == "" {
			t.Errorf("命令 /%s 没有菜单说明", cmd.Command)
		}
	}
}
