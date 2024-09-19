package link_filter

// 链接处理
import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/woodchen-ink/Q58Bot/core"
)

var logger = log.New(log.Writer(), "LinkFilter: ", log.Ldate|log.Ltime|log.Lshortfile)

type LinkFilter struct {
	db          *core.Database
	keywords    []string
	whitelist   []string
	linkPattern *regexp.Regexp
}

// NewLinkFilter 创建一个新的LinkFilter实例。这个实例用于过滤链接，且在创建时会初始化数据库连接和链接过滤正则表达式。
// 它首先尝试创建一个数据库连接，然后加载链接过滤所需的配置，最后返回一个包含所有初始化设置的LinkFilter实例。
// 如果在任何步骤中发生错误，错误将被返回，LinkFilter实例将不会被创建。
func NewLinkFilter() (*LinkFilter, error) {
	// 初始化数据库连接
	db, err := core.NewDatabase()
	if err != nil {
		return nil, err
	}
	// 创建LinkFilter实例
	lf := &LinkFilter{
		db: db,
	}
	// 编译链接过滤正则表达式
	lf.linkPattern = regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`)
	// 从文件中加载额外的链接过滤数据
	err = lf.LoadDataFromFile()
	if err != nil {
		return nil, err
	}
	return lf, nil
}

// LoadDataFromFile 从文件中加载数据到 LinkFilter 结构体的 keywords 和 whitelist 字段。
// 它首先从数据库中获取所有的关键词和白名单条目，如果数据库操作出现错误，它会立即返回错误。
// 一旦数据成功加载，它会通过日志记录加载的关键词和白名单条目的数量。
// 参数: 无
// 返回值: 如果加载过程中发生错误，返回该错误；否则返回 nil。
func (lf *LinkFilter) LoadDataFromFile() error {
	// 从数据库中加载所有关键词到 lf.keywords
	var err error
	lf.keywords, err = lf.db.GetAllKeywords()
	if err != nil {
		// 如果发生错误，立即返回
		return err
	}

	// 从数据库中加载所有白名单条目到 lf.whitelist
	lf.whitelist, err = lf.db.GetAllWhitelist()
	if err != nil {
		// 如果发生错误，立即返回
		return err
	}

	// 记录成功加载的关键词和白名单条目的数量
	logger.Printf("Loaded %d keywords and %d whitelist entries from database", len(lf.keywords), len(lf.whitelist))

	// 数据加载成功，返回 nil
	return nil
}

// NormalizeLink 标准化链接地址。
//
// 该函数接受一个链接字符串，对其进行标准化处理，并返回处理后的链接。
// 标准化过程包括移除协议头（http或https）、TrimPrefix去除链接中的斜杠、
// 解析URL以获取主机名和路径、将查询参数附加到URL末尾。
//
// 参数:
//
//	link - 需要被标准化的链接字符串。
//
// 返回值:
//
//	标准化后的链接字符串。
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

// ExtractDomain 从给定的URL字符串中提取域名。
// 该函数首先解析URL字符串，然后返回解析得到的主机名，同时将其转换为小写。
// 如果URL解析失败，错误信息将被记录，并且函数会返回原始的URL字符串。
// 参数:
//
//	urlStr - 待处理的URL字符串。
//
// 返回值:
//
//	解析后的主机名，如果解析失败则返回原始的URL字符串。
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
	for _, keyword := range lf.keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			logger.Printf("文字包含关键字: %s", text)
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
			found := false
			for _, keyword := range lf.keywords {
				if keyword == normalizedLink {
					logger.Printf("找到现有关键字: %s", normalizedLink)
					return true, nil
				}
			}
			if !found {
				newNonWhitelistedLinks = append(newNonWhitelistedLinks, normalizedLink)
				lf.AddKeyword(normalizedLink)
			}
		}
	}

	if len(newNonWhitelistedLinks) > 0 {
		logger.Printf("发现新的非白名单链接: %v", newNonWhitelistedLinks)
	}
	return false, newNonWhitelistedLinks
}
