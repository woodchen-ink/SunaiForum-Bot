package moderation

import (
	"testing"
	"time"
)

// 真实广告样本, 取自群里实际出现的刷屏消息
var realSpamSamples = []string{
	"1/7/p·m·僅·5·K·多",
	"1·5·ma·x·僅·3·K·多",
	"16·p·m·x·僅·4·K·多",
	"低..出·全·新·水..果·機",
	"水·果·1.6·特·價",
	"·一·臺·搞·3·K",
	"7·折·出·水..果·機·轉·手·搞·3·K",
	"最.新.17·水·果·僅.需.五·千·多",
}

// 正常消息样本, 这些一条都不允许被形态规则判为广告
var legitSamples = []string{
	"今天天气不错，我们去公园吧。",
	"你好，世界，测试，一下，可以吗",
	"服务器 IP 是 192.168.1.100 记得改一下",
	"当前版本 1.2.3，升级到 2.0.0 之后再试",
	"这个价格是 1.6 万，比上次便宜",
	"文档在 https://example.com/docs/getting-started 里",
	"Hello, world. This is a normal English sentence.",
	"BTC 现在多少钱",
	"好的",
	"收到，谢谢",
	"我今天买了一台新的笔记本电脑，用起来还不错",
}

func TestLooksObfuscatedDetectsRealSpam(t *testing.T) {
	for _, sample := range realSpamSamples {
		if !looksObfuscated(sample) {
			t.Errorf("广告样本未被识别: %q", sample)
		}
	}
}

func TestLooksObfuscatedIgnoresLegitText(t *testing.T) {
	for _, sample := range legitSamples {
		if looksObfuscated(sample) {
			t.Errorf("正常消息被误判为广告: %q", sample)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		// 繁体转简体 + 剥离分隔符
		{"水·果·1.6·特·價", "水果16特价"},
		{"低..出·全·新·水..果·機", "低出全新水果机"},
		{"7·折·出·水..果·機·轉·手·搞·3·K", "7折出水果机转手搞3k"},
		{"·一·臺·搞·3·K", "一台搞3k"},
		// 归一化后与正常写法一致, 因此一条关键词可覆盖两种写法
		{"水果特价", "水果特价"},
		// 全角转半角
		{"ＡＢＣ１２３", "abc123"},
		// 零宽字符与 emoji 被丢弃
		{"水​果‍机 🎉", "水果机"},
		{"", ""},
	}

	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// 归一化后, 一条简体关键词应当能命中繁体+分隔符的规避写法
func TestNormalizeEnablesKeywordMatch(t *testing.T) {
	cases := []struct {
		text, keyword string
		wantHit       bool
	}{
		{"水·果·1.6·特·價", "水果", true},
		{"低..出·全·新·水..果·機", "水果机", true},
		{"7·折·出·水..果·機·轉·手·搞·3·K", "转手", true},
		{"最.新.17·水·果·僅.需.五·千·多", "仅需", true},
		{"今天天气不错", "水果", false},
	}

	for _, c := range cases {
		got := matchKeyword(c.text, []string{c.keyword}) != ""
		if got != c.wantHit {
			t.Errorf("matchKeyword(%q, %q) = %v, 期望 %v", c.text, c.keyword, got, c.wantHit)
		}
	}
}

func TestContainsZeroWidth(t *testing.T) {
	if !ContainsZeroWidth("水​果") {
		t.Error("未检出零宽空格")
	}
	if ContainsZeroWidth("正常文本 normal text") {
		t.Error("正常文本被误判含零宽字符")
	}
}

func TestRepeatTrackerFlagsFlooding(t *testing.T) {
	tracker := &repeatTracker{users: make(map[int64]*userHistory)}
	const userID = 42

	text := Normalize("水·果·1.6·特·價")
	for i := 1; i <= repeatThreshold; i++ {
		if got := tracker.record(userID, text); got != i {
			t.Fatalf("第 %d 次记录返回 %d, 期望 %d", i, got, i)
		}
	}

	// 不同内容各自独立计数, 不应相互污染
	if got := tracker.record(userID, Normalize("另一条完全不同的消息内容")); got != 1 {
		t.Errorf("不同内容计数 = %d, 期望 1", got)
	}
	// 不同用户各自独立计数
	if got := tracker.record(userID+1, text); got != 1 {
		t.Errorf("不同用户计数 = %d, 期望 1", got)
	}
}

func TestRepeatTrackerPrune(t *testing.T) {
	tracker := &repeatTracker{users: make(map[int64]*userHistory)}
	tracker.record(1, Normalize("测试一条足够长的消息内容"))

	if removed := tracker.prune(time.Hour); removed != 0 {
		t.Errorf("刚活跃的用户被清理了 %d 个", removed)
	}
	// 用负的空闲阈值确保判定成立: Windows 时钟精度下 time.Since 可能恰好返回 0
	if removed := tracker.prune(-time.Second); removed != 1 {
		t.Errorf("清理数 = %d, 期望 1", removed)
	}
}
