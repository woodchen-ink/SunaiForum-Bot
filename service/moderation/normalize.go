package moderation

// 文本归一化: 把广告常用的规避写法还原成可比较的形式。
//
// 广告通过在每个字之间插入 "·" "." 等分隔符来绕过字面匹配 (水·果·1.6·特·價),
// 并混用繁体与全角字符。归一化后 水·果·1.6·特·價 与 水果1.6特价 得到同一结果, 关键词库无需为每种写法各存一条。
import (
	"strings"
	"unicode"

	"github.com/siongui/gojianfan"
)

// Normalize 归一化文本: 繁体转简体、全角转半角、只保留字母数字与汉字、统一小写。
//
// 标点、空白、emoji、零宽字符全部丢弃, 因此结果**不能**再用于解析链接或展示给用户,
// 只用于关键词匹配与重复内容比对。
func Normalize(text string) string {
	if text == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(text))

	for _, r := range gojianfan.T2S(text) {
		r = toHalfWidth(r)
		// IsLetter 覆盖汉字 (Lo 类); 零宽字符 (Cf)、变体选择符 (Mn)、emoji (So)、标点空白均不满足条件而被丢弃
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}

	return b.String()
}

// toHalfWidth 把全角 ASCII 区 (Ｆ３％ 等) 与全角空格映射回半角
func toHalfWidth(r rune) rune {
	switch {
	case r >= 0xFF01 && r <= 0xFF5E:
		return r - 0xFEE0
	case r == 0x3000:
		return ' '
	default:
		return r
	}
}

// ContainsZeroWidth 判断文本是否含零宽字符或变体选择符。
// 正常输入几乎不会出现这些字符, 它们的存在本身就是刻意规避匹配的信号。
func ContainsZeroWidth(text string) bool {
	for _, r := range text {
		switch {
		case r == 0x200B, r == 0x200C, r == 0x200D, r == 0x2060, r == 0xFEFF:
			return true
		case r >= 0xFE00 && r <= 0xFE0F: // 变体选择符
			return true
		}
	}
	return false
}
