package service

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/woodchen-ink/Q58Bot/core"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var logger = log.New(log.Writer(), "LinkFilter: ", log.Ldate|log.Ltime|log.Lshortfile)

type LinkFilter struct {
	db          *core.Database
	keywords    []string
	whitelist   []string
	linkPattern *regexp.Regexp
}

func NewLinkFilter(dbFile string) (*LinkFilter, error) {
	db, err := core.NewDatabase(dbFile)
	if err != nil {
		return nil, err
	}
	lf := &LinkFilter{
		db: db,
	}
	lf.linkPattern = regexp.MustCompile(`(?i)\b(?:(?:https?://)?(?:(?:www\.)?(?:[a-zA-Z0-9-]+\.)+[a-zA-Z]{2,}|(?:t\.me|telegram\.me))(?:/[^\s]*)?)`)
	err = lf.LoadDataFromFile()
	if err != nil {
		return nil, err
	}
	return lf, nil
}

func (lf *LinkFilter) LoadDataFromFile() error {
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
	link = regexp.MustCompile(`^https?://`).ReplaceAllString(link, "")
	link = strings.TrimPrefix(link, "/")
	parsedURL, err := url.Parse("http://" + link)
	if err != nil {
		logger.Printf("Error parsing URL: %v", err)
		return link
	}
	normalized := fmt.Sprintf("%s%s", parsedURL.Hostname(), parsedURL.EscapedPath())
	if parsedURL.RawQuery != "" {
		normalized += "?" + parsedURL.RawQuery
	}
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
		keywords, err := lf.db.GetAllKeywords()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "获取关键词列表时发生错误。"))
			return
		}
		if len(keywords) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "关键词列表为空。"))
		} else {
			core.SendLongMessage(bot, message.Chat.ID, "当前关键词列表：", keywords)
		}
	case "add":
		if args != "" {
			keyword := args
			exists, err := lf.db.KeywordExists(keyword)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "检查关键词时发生错误。"))
				return
			}
			if !exists {
				err = lf.AddKeyword(keyword)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "添加关键词时发生错误。"))
				} else {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 已添加。", keyword)))
				}
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
				similarKeywords, err := lf.db.SearchKeywords(keyword)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "搜索关键词时发生错误。"))
					return
				}
				if len(similarKeywords) > 0 {
					core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("未找到精确匹配的关键词 '%s'。\n\n以下是相似的关键词：", keyword), similarKeywords)
				} else {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("关键词 '%s' 不存在。", keyword)))
				}
			}
		}
	case "deletecontaining":
		if args != "" {
			substring := args
			removedKeywords, err := lf.RemoveKeywordsContaining(substring)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "删除关键词时发生错误。"))
				return
			}
			if len(removedKeywords) > 0 {
				core.SendLongMessage(bot, message.Chat.ID, fmt.Sprintf("已删除包含 '%s' 的以下关键词：", substring), removedKeywords)
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
		whitelist, err := lf.db.GetAllWhitelist()
		if err != nil {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "获取白名单时发生错误。"))
			return
		}
		if len(whitelist) == 0 {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "白名单为空。"))
		} else {
			bot.Send(tgbotapi.NewMessage(message.Chat.ID, "白名单域名列表：\n"+strings.Join(whitelist, "\n")))
		}
	case "addwhite":
		if args != "" {
			domain := strings.ToLower(args)
			exists, err := lf.db.WhitelistExists(domain)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "检查白名单时发生错误。"))
				return
			}
			if !exists {
				err = lf.db.AddWhitelist(domain)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "添加到白名单时发生错误。"))
					return
				}
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已添加到白名单。", domain)))
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已在白名单中。", domain)))
			}
		}
	case "delwhite":
		if args != "" {
			domain := strings.ToLower(args)
			exists, err := lf.db.WhitelistExists(domain)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, "检查白名单时发生错误。"))
				return
			}
			if exists {
				err = lf.db.RemoveWhitelist(domain)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(message.Chat.ID, "从白名单删除时发生错误。"))
					return
				}
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 已从白名单中删除。", domain)))
			} else {
				bot.Send(tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf("域名 '%s' 不在白名单中。", domain)))
			}
		}
	default:
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "无效的命令或参数。"))
	}
}
