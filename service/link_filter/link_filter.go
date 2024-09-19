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
	db          *core.Database
	keywords    []string
	whitelist   []string
	linkPattern *regexp.Regexp
	mu          sync.RWMutex
}

func NewLinkFilter() (*LinkFilter, error) {
	db, err := core.NewDatabase()
	if err != nil {
		return nil, err
	}

	lf := &LinkFilter{
		db:          db,
		linkPattern: regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`),
	}

	if err := lf.LoadDataFromFile(); err != nil {
		db.Close() // Close the database if loading fails
		return nil, err
	}

	return lf, nil
}

func (lf *LinkFilter) LoadDataFromFile() error {
	lf.mu.Lock()
	defer lf.mu.Unlock()

	var err error
	lf.keywords, err = lf.db.GetAllKeywords()
	if err != nil {
		return err
	}

	lf.whitelist, err = lf.db.GetAllWhitelist()
	if err != nil {
		return err
	}

	logger.Printf("Loaded %d keywords and %d whitelist entries from database", len(lf.keywords), len(lf.whitelist))
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
	for _, whiteDomain := range lf.whitelist {
		if lf.domainMatch(domain, whiteDomain) {
			logger.Printf("Whitelist check for %s: Passed (matched %s)", link, whiteDomain)
			return true
		}
	}
	logger.Printf("Whitelist check for %s: Failed", link)
	return false
}

func (lf *LinkFilter) AddKeyword(keyword string) error {
	if lf.linkPattern.MatchString(keyword) {
		keyword = lf.NormalizeLink(keyword)
	}
	keyword = strings.TrimPrefix(keyword, "/")
	for _, k := range lf.keywords {
		if k == keyword {
			logger.Printf("Keyword already exists: %s", keyword)
			return nil
		}
	}
	err := lf.db.AddKeyword(keyword)
	if err != nil {
		return err
	}
	logger.Printf("New keyword added: %s", keyword)
	return lf.LoadDataFromFile()
}

func (lf *LinkFilter) RemoveKeyword(keyword string) bool {
	for _, k := range lf.keywords {
		if k == keyword {
			lf.db.RemoveKeyword(keyword)
			lf.LoadDataFromFile()
			return true
		}
	}
	return false
}

func (lf *LinkFilter) RemoveKeywordsContaining(substring string) ([]string, error) {
	removed, err := lf.db.RemoveKeywordsContaining(substring)
	if err != nil {
		return nil, err
	}
	err = lf.LoadDataFromFile()
	if err != nil {
		return nil, err
	}
	return removed, nil
}

// 检查消息是否包含关键词或者非白名单链接
func (lf *LinkFilter) ShouldFilter(text string) (bool, []string) {
	logger.Printf("Checking text: %s", text)

	lf.mu.RLock()
	defer lf.mu.RUnlock()

	for _, keyword := range lf.keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			logger.Printf("文字包含关键字: %s", keyword)
			return true, nil
		}
	}

	links := lf.linkPattern.FindAllString(text, -1)
	logger.Printf("找到链接: %v", links)

	var newNonWhitelistedLinks []string
	for _, link := range links {
		normalizedLink := lf.NormalizeLink(link)
		if !lf.IsWhitelisted(normalizedLink) {
			logger.Printf("链接未列入白名单: %s", normalizedLink)
			if !lf.containsKeyword(normalizedLink) {
				newNonWhitelistedLinks = append(newNonWhitelistedLinks, normalizedLink)
				lf.AddKeyword(normalizedLink) // 注意：这里会修改 lf.keywords，可能需要额外的锁
			} else {
				return true, nil
			}
		}
	}

	if len(newNonWhitelistedLinks) > 0 {
		logger.Printf("发现新的非白名单链接: %v", newNonWhitelistedLinks)
	}
	return false, newNonWhitelistedLinks
}

func (lf *LinkFilter) containsKeyword(link string) bool {
	for _, keyword := range lf.keywords {
		if keyword == link {
			return true
		}
	}
	return false
}

// 新增 Close 方法
func (lf *LinkFilter) Close() error {
	return lf.db.Close()
}
