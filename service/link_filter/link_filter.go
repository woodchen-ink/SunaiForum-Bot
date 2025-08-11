package link_filter

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"SunaiForum-Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

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
		return nil, fmt.Errorf("failed to load data from database: %v", err)
	}

	return lf, nil
}

func (lf *LinkFilter) LoadDataFromDatabase() error {
	lf.Mu.Lock()
	defer lf.Mu.Unlock()

	var err error
	lf.Keywords, err = core.DB.GetAllKeywords()
	if err != nil {
		return fmt.Errorf("failed to get keywords: %v", err)
	}

	lf.Whitelist, err = core.DB.GetAllWhitelist()
	if err != nil {
		return fmt.Errorf("failed to get whitelist: %v", err)
	}

	log.Printf("[LinkFilter] Loaded %d Keywords and %d Whitelist entries from database", len(lf.Keywords), len(lf.Whitelist))
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
		log.Printf("[LinkFilter] Error parsing URL: %v", err)
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
	log.Printf("[LinkFilter] Normalized link: %s -> %s", link, result)
	return result
}

func (lf *LinkFilter) ExtractDomain(urlStr string) string {
	// 尝试解析给定的URL字符串。
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// 如果解析过程中出现错误，记录错误信息并返回原始URL字符串。
		log.Printf("[LinkFilter] Error parsing URL: %v", err)
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
			log.Printf("[LinkFilter] Whitelist check for %s: Passed (matched %s)", link, whiteDomain)
			return true
		}
	}
	log.Printf("[LinkFilter] Whitelist check for %s: Failed", link)
	return false
}

func addNewKeyword(keyword string) error {
	exists, err := core.DB.KeywordExists(keyword)
	if err != nil {
		return fmt.Errorf("检查关键词时发生错误: %v", err)
	}
	if !exists {
		err = core.DB.AddKeyword(keyword, true, true) // isLink = true, isAutoAdded = true
		if err != nil {
			return fmt.Errorf("添加关键词时发生错误: %v", err)
		}
		log.Printf("[LinkFilter] 新关键词已添加: %s", keyword)
	}
	return nil
}

func containsKeyword(text string, linkFilter *LinkFilter) bool {
	linkFilter.Mu.RLock()
	defer linkFilter.Mu.RUnlock()

	for _, keyword := range linkFilter.Keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			log.Printf("[LinkFilter] 文字包含关键字: %s", keyword)
			return true
		}
	}
	return false
}

func extractLinks(text string, linkFilter *LinkFilter) []string {
	linkFilter.Mu.RLock()
	defer linkFilter.Mu.RUnlock()

	links := linkFilter.LinkPattern.FindAllString(text, -1)
	log.Printf("[LinkFilter] 找到链接: %v", links)
	return links
}
func ShouldFilter(text string, linkFilter *LinkFilter) (bool, []string) {
	log.Printf("[LinkFilter] Checking text: %s", text)
	if len(text) > 200 {
		text = text[:200]
	}
	log.Printf("[LinkFilter] 检查文本: %s", text)

	if containsKeyword(text, linkFilter) {
		return true, nil
	}

	links := extractLinks(text, linkFilter)
	return processLinks(links, linkFilter)
}
func processLinks(links []string, linkFilter *LinkFilter) (bool, []string) {
	var newNonWhitelistedLinks []string

	for _, link := range links {
		normalizedLink := linkFilter.NormalizeLink(link)
		if !linkFilter.IsWhitelisted(normalizedLink) {
			log.Printf("[LinkFilter] 链接未列入白名单: %s", normalizedLink)
			if !containsKeyword(normalizedLink, linkFilter) {
				newNonWhitelistedLinks = append(newNonWhitelistedLinks, normalizedLink)
				err := addNewKeyword(normalizedLink)
				if err != nil {
					log.Printf("[LinkFilter] 添加关键词时发生错误: %v", err)
				}
				// 如果成功添加了新关键词，更新 linkFilter 的 Keywords
				linkFilter.Mu.Lock()
				linkFilter.Keywords = append(linkFilter.Keywords, normalizedLink)
				linkFilter.Mu.Unlock()
			} else {
				return true, nil
			}
		}
	}

	return false, newNonWhitelistedLinks
}
func (lf *LinkFilter) CheckAndFilterLink(bot *tgbotapi.BotAPI, message *tgbotapi.Message) bool {
	// 判断消息是否应当被过滤及找出新的非白名单链接
	shouldFilter, newLinks := ShouldFilter(message.Text, lf)

	// 如果发现新的非白名单链接，记录日志
	if len(newLinks) > 0 {
		log.Printf("[LinkFilter] 发现新的非白名单链接: %v", newLinks)
	}

	if shouldFilter {
		// 记录被过滤的消息
		log.Printf("[LinkFilter] 消息应该被过滤: %s", message.Text)
		// 删除原始消息
		deleteMsg := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
		_, err := bot.Request(deleteMsg)
		if err != nil {
			// 删除消息失败时记录错误
			log.Printf("[LinkFilter] 删除消息失败: %v", err)
			return true
		}

		// 发送提示消息
		notification := tgbotapi.NewMessage(message.Chat.ID, "已撤回该消息。注:一个链接不能发两次.")
		sent, err := bot.Send(notification)
		if err != nil {
			// 发送通知失败时记录错误
			log.Printf("[LinkFilter] 发送通知失败: %v", err)
		} else {
			// 3分钟后删除提示消息
			core.DeleteMessageAfterDelay(bot, message.Chat.ID, sent.MessageID, 3*time.Minute)
		}
		return true
	}

	return false
}
