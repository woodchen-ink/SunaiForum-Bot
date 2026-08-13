package core

// 环境变量解析的边界。
// 这些用例来自真实踩到的坑: Dokploy 面板里粘贴的值常带尾随空格,
// 而"值后面写 # 注释"是很自然的习惯, 但 Docker env 格式并不把它当注释。
import "testing"

func TestCleanEnvValue(t *testing.T) {
	cases := []struct{ in, want string }{
		{"3", "3"},
		{"5912366993 ", "5912366993"}, // 面板粘贴常见的尾随空格
		{"3             # 累计违规几次自动封禁", "3"}, // 行内注释
		{"true     # 自动清理加入/退出群组通知", "true"},
		{"  0  # 观察期  ", "0"},
		{"# 整行注释", ""},
		{"", ""},
	}

	for _, c := range cases {
		if got := cleanEnvValue(c.in); got != c.want {
			t.Errorf("cleanEnvValue(%q) = %q, 期望 %q", c.in, got, c.want)
		}
	}
}

// TestParseInt64TrimsWhitespace 尾随空格不能让必填 ID 解析失败 —— 那会导致进程启动即退出
func TestParseInt64TrimsWhitespace(t *testing.T) {
	for _, raw := range []string{"5912366993", "5912366993 ", " 5912366993", "-1002228579781 "} {
		if _, err := parseInt64(raw); err != nil {
			t.Errorf("parseInt64(%q) 报错: %v", raw, err)
		}
	}

	if _, err := parseInt64("  "); err == nil {
		t.Error("纯空白应当报错")
	}
	if _, err := parseInt64("abc"); err == nil {
		t.Error("非数字应当报错")
	}
}

// TestParseIntEnvIgnoresInlineComment 带注释的值要解析出真实数字, 而不是静默回落默认值。
// 回落默认值最危险的场景: 写 AUTO_BAN_THRESHOLD=0 想关掉自动封禁, 却回落成 3 把它开着。
func TestParseIntEnvIgnoresInlineComment(t *testing.T) {
	t.Setenv("TEST_INT", "0   # 观察期先不封禁")
	if got := parseIntEnv("TEST_INT", 3); got != 0 {
		t.Errorf("parseIntEnv = %d, 期望 0 (不能回落到默认值 3)", got)
	}

	t.Setenv("TEST_INT", "200  # 每小时上限")
	if got := parseIntEnv("TEST_INT", 100); got != 200 {
		t.Errorf("parseIntEnv = %d, 期望 200", got)
	}

	// 真正非法的值仍然回落默认
	t.Setenv("TEST_INT", "abc")
	if got := parseIntEnv("TEST_INT", 7); got != 7 {
		t.Errorf("非法值应回落默认 7, 得到 %d", got)
	}
}

func TestParseFloatEnvIgnoresInlineComment(t *testing.T) {
	t.Setenv("TEST_FLOAT", "0.8  # 低于此置信度不处置")
	if got := parseFloatEnv("TEST_FLOAT", 0.5); got != 0.8 {
		t.Errorf("parseFloatEnv = %v, 期望 0.8", got)
	}

	// 置信度必须落在 0..1
	t.Setenv("TEST_FLOAT", "1.5")
	if got := parseFloatEnv("TEST_FLOAT", 0.8); got != 0.8 {
		t.Errorf("越界值应回落默认 0.8, 得到 %v", got)
	}
}

func TestParseBoolEnv(t *testing.T) {
	cases := []struct {
		raw      string
		fallback bool
		want     bool
	}{
		{"true     # 自动清理群务通知", true, true},
		{"false    # 关掉", true, false},
		{"0  # 关掉", true, false},
		{"off", true, false},
		{"no", true, false},
		{"", true, true},
		{"", false, false},
		{"yes", false, true},
	}

	for _, c := range cases {
		t.Setenv("TEST_BOOL", c.raw)
		if got := parseBoolEnv("TEST_BOOL", c.fallback); got != c.want {
			t.Errorf("parseBoolEnv(%q, fallback=%v) = %v, 期望 %v", c.raw, c.fallback, got, c.want)
		}
	}
}

func TestParseSymbols(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"DOGS/USDT,TON/USDT", 2},
		{" DOGS/USDT , TON/USDT ", 2},
		{"", 0}, // 未配置时必须是空切片, 否则币安模块会当成"配了一个空交易对"
		{",,,", 0},
	}

	for _, c := range cases {
		if got := parseSymbols(c.in); len(got) != c.want {
			t.Errorf("parseSymbols(%q) = %v, 期望 %d 项", c.in, got, c.want)
		}
	}

	if got := parseSymbols("DOGS/USDT"); len(got) != 1 || got[0] != "DOGSUSDT" {
		t.Errorf("parseSymbols 未去掉斜杠: %v", got)
	}
}
