package link_filter

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/woodchen-ink/Q58Bot/core"
)

var logger = log.New(log.Writer(), "LinkFilter: ", log.Ldate|log.Ltime|log.Lshortfile)

type LinkFilter struct {
	Keywords    []string
	Whitelist   []string
	LinkPattern *regexp.Regexp
	Mu          sync.RWMutex
}

func NewLinkFilter() (*LinkFilter, error) {
	lf := &LinkFilter{
		LinkPattern: regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`),
	}

	if err := lf.LoadDataFromDatabase(); err != nil {
		return nil, err
	}

	return lf, nil
}

func (lf *LinkFilter) LoadDataFromDatabase() error {
	lf.Mu.Lock()
	defer lf.Mu.Unlock()

	var err error
	lf.Keywords, err = core.DB.GetAllKeywords()
	if err != nil {
		return err
	}

	lf.Whitelist, err = core.DB.GetAllWhitelist()
	if err != nil {
		return err
	}

	logger.Printf("Loaded %d Keywords and %d Whitelist entries from database", len(lf.Keywords), len(lf.Whitelist))
	return nil
}

func (lf *LinkFilter) NormalizeLink(link string) string {
	// 移除链接中的协议头（http或https）
	link = regexp.MustCompile(`^https?://`).ReplaceAllString(link, "")
	// 去除链接中的斜杠
	link = strings.TrimPrefix(link, "/")
	// 解析URL，此处默认使用http协议，因为协议头部已被移除
	parsedURL, err := url.Parse("http://" + link)
	if err != nil {
		// 如果URL解析失败，记录错误信息，并返回原始链接
		logger.Printf("Error parsing URL: %v", err)
		return link
	}
	// 构建标准化的URL，包含主机名和转义后的路径
	normalized := fmt.Sprintf("%s%s", parsedURL.Hostname(), parsedURL.EscapedPath())
	// 如果URL有查询参数，将其附加到标准化的URL后面
	if parsedURL.RawQuery != "" {
		normalized += "?" + parsedURL.RawQuery
	}
	// 移除标准化URL末尾的斜杠（如果有）
	result := strings.TrimSuffix(normalized, "/")
	// 记录标准化后的链接信息
	logger.Printf("Normalized link: %s -> %s", link, result)
	return result
}

func (lf *LinkFilter) ExtractDomain(urlStr string) string {
	// 尝试解析给定的URL字符串。
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// 如果解析过程中出现错误，记录错误信息并返回原始URL字符串。
		logger.Printf("Error parsing URL: %v", err)
		return urlStr
	}
	// 返回解析得到的主机名，转换为小写。
	return strings.ToLower(parsedURL.Hostname())
}

func (lf *LinkFilter) domainMatch(domain, whiteDomain string) bool {
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
func (lf *LinkFilter) IsWhitelisted(link string) bool {
	domain := lf.ExtractDomain(link)
	for _, whiteDomain := range lf.Whitelist {
		if lf.domainMatch(domain, whiteDomain) {
			logger.Printf("Whitelist check for %s: Passed (matched %s)", link, whiteDomain)
			return true
		}
	}
	logger.Printf("Whitelist check for %s: Failed", link)
	return false
}
