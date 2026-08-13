package moderation

// 形态规则: 不依赖词库, 只看文本的写法特征和发送节奏。
// 广告可以随时换词, 但"逐字插分隔符"和"定时重复刷屏"这两个形态很难放弃, 因此这类规则比关键词更耐用。
import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// 分隔符混淆判定阈值: 可疑分隔符达到 3 处, 且占正文字符比例超过 1/4
	minObfuscationHits  = 3
	minObfuscationRatio = 0.25
	// minContentRunes 正文字符太少时形态判断不可靠, 直接跳过
	minContentRunes = 5

	// 同一用户在 repeatWindow 内发出 repeatThreshold 条内容相同的消息即判定刷屏
	repeatThreshold = 3
	repeatWindow    = 10 * time.Minute
	// minRepeatLength 归一化后太短的内容 ("好" "收到") 重复属正常聊天, 不计入刷屏
	minRepeatLength = 6
	// maxHistoryPerUser 每个用户保留的最近消息条数上限
	maxHistoryPerUser = 20
)

// decorativeSeparators 广告用来拆字的分隔符。
// 中文标点 (，。、！？；：) 不在其中 —— 那是正常书写, 计入会大量误判。
const decorativeSeparators = "·•∙‧・･．.,-_*~/\\|+=^"

// looksObfuscated 检测"逐字插入分隔符"的规避写法, 例如 水·果·1.6·特·價。
//
// 判定思路: 把文本切成"正文段"与"分隔段", 只统计夹在两个正文段之间、且不含空格的分隔段。
// 不含空格是关键 —— 正常书写 "Hello, world" 的逗号后有空格, 而广告的 "水·果" 没有。
// 纯数字之间的 "." "," 额外放行, 避免误伤小数、IP 和版本号。
func looksObfuscated(text string) bool {
	runes := []rune(text)

	var contentCount, suspiciousCount int
	i := 0
	for i < len(runes) {
		if isContentRune(runes[i]) {
			contentCount++
			i++
			continue
		}

		// 收集一整段连续分隔符, 记下它两侧的正文字符
		start := i
		for i < len(runes) && !isContentRune(runes[i]) {
			i++
		}
		if start == 0 || i >= len(runes) {
			continue // 位于文本首尾, 不是"夹在中间"
		}

		if isSuspiciousSeparator(runes[start:i], runes[start-1], runes[i]) {
			suspiciousCount++
		}
	}

	if contentCount < minContentRunes || suspiciousCount < minObfuscationHits {
		return false
	}
	return float64(suspiciousCount)/float64(contentCount) >= minObfuscationRatio
}

// isContentRune 判断是否为正文字符 (字母、数字、汉字)
func isContentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isSuspiciousSeparator 判断一段分隔符是否属于刻意拆字, prev / next 为其两侧的正文字符
func isSuspiciousSeparator(sep []rune, prev, next rune) bool {
	hasDecorative := false
	onlyDotComma := true

	for _, r := range sep {
		if unicode.IsSpace(r) {
			return false // 带空格的是正常书写
		}
		if strings.ContainsRune(decorativeSeparators, r) {
			hasDecorative = true
		}
		if r != '.' && r != ',' {
			onlyDotComma = false
		}
	}

	if !hasDecorative {
		return false
	}
	// 放行小数、IP、版本号: 纯数字之间的 "." 或 ","
	if onlyDotComma && unicode.IsDigit(prev) && unicode.IsDigit(next) {
		return false
	}
	return true
}

// contactPatterns 引流话术特征; 命中不直接判广告, 只作为"值得花一次 AI 判定"的弱信号
var contactPatterns = []string{
	"私聊", "加我", "微信", "威信", "vx", "wx", "qq", "扣扣",
	"联系我", "简介", "主页", "详聊", "телеграм", "dm我", "pm我",
	"收款", "代收", "刷单", "兼职", "日入", "月入", "回收", "出售", "特价", "低价",
}

// pricePattern 数字紧跟金额单位, 例如 3K / 5000元 / 1.6万
var pricePattern = regexp.MustCompile(`(?i)\d+\s*(k|w|元|块|万|千|刀|u|usdt)`)

// HasWeakSignal 判断文本是否值得花一次 AI 判定。
//
// 这是成本闸门, 不是判定依据: 命中只代表"可疑到值得看一眼", 单独命中不构成拦截理由。
// 阈值刻意放宽 —— 漏掉一次 AI 判定的代价, 远大于多花一次调用。
func HasWeakSignal(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}

	normalized := Normalize(text)
	for _, pattern := range contactPatterns {
		if strings.Contains(normalized, Normalize(pattern)) {
			return true
		}
	}

	if pricePattern.MatchString(text) {
		return true
	}
	if len(ExtractLinks(text)) > 0 {
		return true
	}
	// 有拆字迹象但没到判定阈值的, 也送 AI 看一眼
	return countSuspiciousSeparators(text) > 0
}

// countSuspiciousSeparators 统计可疑分隔符段数, 供弱信号判断复用
func countSuspiciousSeparators(text string) int {
	runes := []rune(text)

	count := 0
	i := 0
	for i < len(runes) {
		if isContentRune(runes[i]) {
			i++
			continue
		}

		start := i
		for i < len(runes) && !isContentRune(runes[i]) {
			i++
		}
		if start == 0 || i >= len(runes) {
			continue
		}
		if isSuspiciousSeparator(runes[start:i], runes[start-1], runes[i]) {
			count++
		}
	}
	return count
}

// repeatTracker 按用户记录最近发过的内容, 用于识别定时重复刷屏。
// 只放内存: 刷屏判定看的是 10 分钟内的节奏, 重启后重新观察即可, 不值得落库。
type repeatTracker struct {
	mu    sync.Mutex
	users map[int64]*userHistory
}

type userHistory struct {
	recent   []textStamp
	lastSeen time.Time
}

type textStamp struct {
	text string // 归一化后的内容
	at   time.Time
}

var repeats = &repeatTracker{users: make(map[int64]*userHistory)}

// record 记下一条消息并返回该内容在时间窗内出现的总次数 (含本条)
func (t *repeatTracker) record(userID int64, normalized string) int {
	if len(normalized) == 0 {
		return 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	history, ok := t.users[userID]
	if !ok {
		history = &userHistory{}
		t.users[userID] = history
	}
	history.lastSeen = now

	// 丢弃超出时间窗的旧记录, 顺便统计同内容出现次数
	kept := history.recent[:0]
	count := 1
	for _, entry := range history.recent {
		if now.Sub(entry.at) > repeatWindow {
			continue
		}
		if entry.text == normalized {
			count++
		}
		kept = append(kept, entry)
	}

	kept = append(kept, textStamp{text: normalized, at: now})
	if len(kept) > maxHistoryPerUser {
		kept = kept[len(kept)-maxHistoryPerUser:]
	}
	history.recent = kept

	return count
}

// prune 清掉长期不活跃用户的记录, 由定时任务调用, 防止 map 无限增长
func (t *repeatTracker) prune(idleFor time.Duration) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	removed := 0
	for userID, history := range t.users {
		if time.Since(history.lastSeen) > idleFor {
			delete(t.users, userID)
			removed++
		}
	}
	return removed
}

// PruneRepeatHistory 清理不活跃用户的刷屏记录, 返回清理条数
func PruneRepeatHistory(idleFor time.Duration) int {
	return repeats.prune(idleFor)
}
