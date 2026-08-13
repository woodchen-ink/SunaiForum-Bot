package core

// 管理员输入的校验规则; 所有写库操作前都必须先过这里
import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxKeywordLength = 100
	maxDomainLength  = 253
	maxPromptLength  = 100
	maxReplyLength   = 1000
)

// forbiddenChars 关键词与提示词中不接受的字符。
// 查询全部走参数化占位符, 这里拦截的是引号反斜杠带来的展示与日志歧义, 不承担防注入职责。
const forbiddenChars = "';\"\\"

// domainPattern RFC 1123 形式的域名, 逐段限制 63 字符
var domainPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateKeyword 校验过滤关键词; 长度按字符数而非字节数计, 避免中文被误判超长
func ValidateKeyword(keyword string) error {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return fmt.Errorf("关键词不能为空")
	}

	if utf8.RuneCountInString(keyword) > maxKeywordLength {
		return fmt.Errorf("关键词长度不能超过 %d 个字符", maxKeywordLength)
	}

	if strings.ContainsAny(keyword, forbiddenChars) {
		return fmt.Errorf("关键词包含不允许的字符")
	}

	return nil
}

// ValidateDomain 校验白名单域名格式
func ValidateDomain(domain string) error {
	domain = strings.TrimSpace(strings.ToLower(domain))
	if domain == "" {
		return fmt.Errorf("域名不能为空")
	}

	if len(domain) > maxDomainLength {
		return fmt.Errorf("域名长度不能超过 %d 个字符", maxDomainLength)
	}

	if !domainPattern.MatchString(domain) {
		return fmt.Errorf("域名格式无效")
	}

	return nil
}

// ValidatePrompt 校验提示词与其自动回复内容
func ValidatePrompt(prompt, reply string) error {
	prompt = strings.TrimSpace(prompt)
	reply = strings.TrimSpace(reply)

	if prompt == "" {
		return fmt.Errorf("提示词不能为空")
	}
	if reply == "" {
		return fmt.Errorf("回复内容不能为空")
	}

	if utf8.RuneCountInString(prompt) > maxPromptLength {
		return fmt.Errorf("提示词长度不能超过 %d 个字符", maxPromptLength)
	}
	if utf8.RuneCountInString(reply) > maxReplyLength {
		return fmt.Errorf("回复内容长度不能超过 %d 个字符", maxReplyLength)
	}

	if strings.ContainsAny(prompt, forbiddenChars) || strings.ContainsAny(reply, forbiddenChars) {
		return fmt.Errorf("提示词和回复内容包含不允许的字符")
	}

	return nil
}
