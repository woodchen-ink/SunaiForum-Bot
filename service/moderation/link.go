package moderation

// 链接提取与白名单匹配。
//
// 现状: 本文件的函数暂无调用方。"同一链接不能发两次"功能已下线, 原有的链接拦截随之移除,
// 但 /addwhite /delwhite /listwhite 命令与 whitelist 表仍在维护数据。
// 这些函数保留给下一批的"新用户前 N 条消息禁发非白名单外链", 届时由 filter.go 调用;
// 若该需求取消, 应连同白名单命令与 whitelist 表一起删除, 不要单独留着。
import (
	"log"
	"net/url"
	"regexp"
	"strings"

	"SunaiForum-Bot/core"
)

// linkPattern 匹配裸域名与带协议的 URL, 含 Telegram 短链
var linkPattern = regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`)

// schemePattern 用于剥离链接开头的协议部分
var schemePattern = regexp.MustCompile(`^https?://`)

// ExtractLinks 从文本中提取所有疑似链接
func ExtractLinks(text string) []string {
	return linkPattern.FindAllString(text, -1)
}

// NormalizeLink 去掉协议头与末尾斜杠, 得到 主机名+路径[?查询] 形式的规范链接
func NormalizeLink(link string) string {
	link = schemePattern.ReplaceAllString(link, "")
	link = strings.TrimPrefix(link, "/")

	parsedURL, err := url.Parse("http://" + link)
	if err != nil {
		log.Printf("[Moderation] 解析链接失败: %v", err)
		return link
	}

	normalized := parsedURL.Hostname() + parsedURL.EscapedPath()
	if parsedURL.RawQuery != "" {
		normalized += "?" + parsedURL.RawQuery
	}
	return strings.TrimSuffix(normalized, "/")
}

// extractDomain 取链接的主机名, 统一小写
func extractDomain(urlStr string) string {
	parsedURL, err := url.Parse("http://" + strings.TrimPrefix(urlStr, "http://"))
	if err != nil {
		log.Printf("[Moderation] 解析域名失败: %v", err)
		return strings.ToLower(urlStr)
	}
	return strings.ToLower(parsedURL.Hostname())
}

// IsWhitelisted 判断链接的域名是否命中白名单; 白名单项按后缀匹配, 因此父域名可覆盖其子域名。
// 查库失败时返回 true, 宁可放行也不误删。
func IsWhitelisted(link string) bool {
	whitelist, err := core.DB.GetAllWhitelist()
	if err != nil {
		log.Printf("[Moderation] 读取白名单失败, 按放行处理: %v", err)
		return true
	}

	domain := extractDomain(link)
	for _, whiteDomain := range whitelist {
		if domainMatch(domain, whiteDomain) {
			return true
		}
	}
	return false
}

// domainMatch 判断 domain 是否等于 whiteDomain 或为其子域名
func domainMatch(domain, whiteDomain string) bool {
	domainParts := strings.Split(domain, ".")
	whiteDomainParts := strings.Split(whiteDomain, ".")

	if len(domainParts) < len(whiteDomainParts) {
		return false
	}

	for i := 1; i <= len(whiteDomainParts); i++ {
		if domainParts[len(domainParts)-i] != whiteDomainParts[len(whiteDomainParts)-i] {
			return false
		}
	}

	return true
}
