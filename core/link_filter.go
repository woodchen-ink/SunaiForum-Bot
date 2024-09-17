package core

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var logger = log.New(log.Writer(), "LinkFilter: ", log.Ldate|log.Ltime|log.Lshortfile)

type LinkFilter struct {
	db          *Database
	keywords    []string
	whitelist   []string
	linkPattern *regexp.Regexp
}

func NewLinkFilter(dbFile string) *LinkFilter {
	lf := &LinkFilter{
		db: NewDatabase(dbFile),
	}
	lf.linkPattern = regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`)
	lf.LoadDataFromFile()
	return lf
}

func (lf *LinkFilter) LoadDataFromFile() {
	lf.keywords = lf.db.GetAllKeywords()
	lf.whitelist = lf.db.GetAllWhitelist()
	logger.Printf("Loaded %d keywords and %d whitelist entries from database", len(lf.keywords), len(lf.whitelist))
}

func (lf *LinkFilter) NormalizeLink(link string) string {
	link = regexp.MustCompile(`^https?://`).ReplaceAllString(link, "")
	link = strings.TrimPrefix(link, "/")
	parsedURL, err := url.Parse("http://" + link)
	if err != nil {
		logger.Printf("Error parsing URL: %v", err)
		return link
	}
	normalized := fmt.Sprintf("%s%s", parsedURL.Hostname(), parsedURL.EscapedPath())
	result := strings.TrimSuffix(normalized, "/")
	logger.Printf("Normalized link: %s -> %s", link, result)
	return result
}

func (lf *LinkFilter) ExtractDomain(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		logger.Printf("Error parsing URL: %v", err)
		return urlStr
	}
	domain := parsedURL.Hostname()
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		domain = strings.Join(parts[len(parts)-2:], ".")
	}
	return strings.ToLower(domain)
}

func (lf *LinkFilter) IsWhitelisted(link string) bool {
	domain := lf.ExtractDomain(link)
	for _, whiteDomain := range lf.whitelist {
		if domain == whiteDomain {
			logger.Printf("Whitelist check for %s: Passed", link)
			return true
		}
	}
	logger.Printf("Whitelist check for %s: Failed", link)
	return false
}

func (lf *LinkFilter) AddKeyword(keyword string) {
	if lf.linkPattern.MatchString(keyword) {
		keyword = lf.NormalizeLink(keyword)
	}
	keyword = strings.TrimPrefix(keyword, "/")
	for _, k := range lf.keywords {
		if k == keyword {
			logger.Printf("Keyword already exists: %s", keyword)
			return
		}
	}
	lf.db.AddKeyword(keyword)
	logger.Printf("New keyword added: %s", keyword)
	lf.LoadDataFromFile()
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

func (lf *LinkFilter) RemoveKeywordsContaining(substring string) []string {
	removed := lf.db.RemoveKeywordsContaining(substring)
	lf.LoadDataFromFile()
	return removed
}

func (lf *LinkFilter) ShouldFilter(text string) (bool, []string) {
	logger.Printf("Checking text: %s", text)
	for _, keyword := range lf.keywords {
		if strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			logger.Printf("Text contains keyword: %s", text)
			return true, nil
		}
	}

	links := lf.linkPattern.FindAllString(text, -1)
	logger.Printf("Found links: %v", links)
	var newNonWhitelistedLinks []string
	for _, link := range links {
		normalizedLink := lf.NormalizeLink(link)
		normalizedLink = strings.TrimPrefix(normalizedLink, "/")
		if !lf.IsWhitelisted(normalizedLink) {
			logger.Printf("Link not whitelisted: %s", normalizedLink)
			found := false
			for _, keyword := range lf.keywords {
				if keyword == normalizedLink {
					logger.Printf("Existing keyword found: %s", normalizedLink)
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
		logger.Printf("New non-whitelisted links found: %v", newNonWhitelistedLinks)
	}
	return false, newNonWhitelistedLinks
}

func (lf *LinkFilter) HandleKeywordCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string) {
	switch command {
	case "list":
		keywords := lf.db.GetAllKeywords()
		if len(keywords) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "关键词列表为空。"))
		} else {
			SendLongMessage(bot, message.Chat.ID, "当前关键词列表：", keywords)
		}
	case "add":
		if args != "" {
			keyword := args
			if !lf.db.KeywordExists(keyword) {
				lf.AddKeyword(keyword)
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword)))
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已存在。", keyword)))
			}
		}
	case "delete":
		if args != "" {
			keyword := args
			if lf.RemoveKeyword(keyword) {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已删除。", keyword)))
			} else {
				similarKeywords := lf.db.SearchKeywords(keyword)
				if len(similarKeywords) > 0 {
					SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未找到精确匹配的关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
				} else {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 不存在。", keyword)))
				}
			}
		}
	case "deletecontaining":
		if args != "" {
			substring := args
			removedKeywords := lf.RemoveKeywordsContaining(substring)
			if len(removedKeywords) > 0 {
				SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removedKeywords)
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("没有找到包含 '%s' 的关键词。", substring)))
			}
		}
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "无效的命令或参数。"))
	}
}

func (lf *LinkFilter) HandleWhitelistCommand(bot *tgbotapi.BotAPI, message *tgbotapi.Message, command string, args string) {
	switch command {
	case "listwhite":
		whitelist := lf.db.GetAllWhitelist()
		if len(whitelist) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "白名单为空。"))
		} else {
			SendLongMessageWithoutNumbering(bot, message.Chat.ID, "白名单域名列表：", whitelist)
		}
	case "addwhite":
		if args != "" {
			domain := strings.ToLower(args)
			if !lf.db.WhitelistExists(domain) {
				lf.db.AddWhitelist(domain)
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已添加到白名单。", domain)))
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain)))
			}
		}
	case "delwhite":
		if args != "" {
			domain := strings.ToLower(args)
			if lf.db.WhitelistExists(domain) {
				lf.db.RemoveWhitelist(domain)
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已从白名单中删除。", domain)))
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain)))
			}
		}
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "无效的命令或参数。"))
	}
}
