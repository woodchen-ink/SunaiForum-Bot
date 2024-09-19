package core

//数据库处理
import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db                     *sql.DB
	keywordsCache          []string
	whitelistCache         []string
	promptRepliesCache     map[string]string
	keywordsCacheTime      time.Time
	whitelistCacheTime     time.Time
	promptRepliesCacheTime time.Time
	mu                     sync.Mutex
}

func NewDatabase() (*Database, error) {
	os.MkdirAll(filepath.Dir(DB_FILE), os.ModePerm)
	db, err := sql.Open("sqlite", DB_FILE)
	if err != nil {
		return nil, err
	}

	database := &Database{
		db: db,
	}

	if err := database.createTables(); err != nil {
		return nil, err
	}

	return database, nil
}

func (d *Database) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS keywords
			 (id INTEGER PRIMARY KEY, keyword TEXT UNIQUE)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword ON keywords(keyword)`,
		`CREATE TABLE IF NOT EXISTS whitelist
			 (id INTEGER PRIMARY KEY, domain TEXT UNIQUE)`,
		`CREATE INDEX IF NOT EXISTS idx_domain ON whitelist(domain)`,
		`CREATE TABLE IF NOT EXISTS prompt_replies
             (prompt TEXT PRIMARY KEY, reply TEXT NOT NULL)`,
	}

	for _, query := range queries {
		_, err := d.db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *Database) executeQuery(query string, args ...interface{}) ([]string, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (d *Database) AddKeyword(keyword string) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO keywords (keyword) VALUES (?)", keyword)
	if err != nil {
		return err
	}
	d.invalidateCache("keywords")
	return nil
}

func (d *Database) RemoveKeyword(keyword string) (bool, error) {
	result, err := d.db.Exec("DELETE FROM keywords WHERE keyword = ?", keyword)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	d.invalidateCache("keywords")
	return rowsAffected > 0, nil
}

func (d *Database) GetAllKeywords() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.keywordsCache == nil || time.Since(d.keywordsCacheTime) > 5*time.Minute {
		keywords, err := d.executeQuery("SELECT keyword FROM keywords")
		if err != nil {
			return nil, err
		}
		d.keywordsCache = keywords
		d.keywordsCacheTime = time.Now()
	}

	return d.keywordsCache, nil
}

func (d *Database) RemoveKeywordsContaining(substring string) ([]string, error) {
	// 首先获取要删除的关键词列表
	rows, err := d.db.Query("SELECT keyword FROM keywords WHERE keyword LIKE ?", "%"+substring+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var removedKeywords []string
	for rows.Next() {
		var keyword string
		if err := rows.Scan(&keyword); err != nil {
			return nil, err
		}
		removedKeywords = append(removedKeywords, keyword)
	}

	// 执行删除操作
	_, err = d.db.Exec("DELETE FROM keywords WHERE keyword LIKE ?", "%"+substring+"%")
	if err != nil {
		return nil, err
	}

	d.invalidateCache("keywords")
	return removedKeywords, nil
}

func (d *Database) AddWhitelist(domain string) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO whitelist (domain) VALUES (?)", domain)
	if err != nil {
		return err
	}
	d.invalidateCache("whitelist")
	return nil
}

func (d *Database) RemoveWhitelist(domain string) error {
	_, err := d.db.Exec("DELETE FROM whitelist WHERE domain = ?", domain)
	if err != nil {
		return err
	}
	d.invalidateCache("whitelist")
	return nil
}

func (d *Database) GetAllWhitelist() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.whitelistCache == nil || time.Since(d.whitelistCacheTime) > 5*time.Minute {
		whitelist, err := d.executeQuery("SELECT domain FROM whitelist")
		if err != nil {
			return nil, err
		}
		d.whitelistCache = whitelist
		d.whitelistCacheTime = time.Now()
	}

	return d.whitelistCache, nil
}

func (d *Database) SearchKeywords(pattern string) ([]string, error) {
	return d.executeQuery("SELECT keyword FROM keywords WHERE keyword LIKE ?", "%"+pattern+"%")
}

func (d *Database) KeywordExists(keyword string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM keywords WHERE keyword = ?", keyword).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) WhitelistExists(domain string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM whitelist WHERE domain = ?", domain).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *Database) AddPromptReply(prompt, reply string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT OR REPLACE INTO prompt_replies (prompt, reply) VALUES (?, ?)", strings.ToLower(prompt), reply)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	d.invalidateCache("promptReplies")
	return nil
}

func (d *Database) DeletePromptReply(prompt string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM prompt_replies WHERE prompt = ?", strings.ToLower(prompt))
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	d.invalidateCache("promptReplies")
	return nil
}

func (d *Database) fetchAllPromptReplies() (map[string]string, error) {
	rows, err := d.db.Query("SELECT prompt, reply FROM prompt_replies")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promptReplies := make(map[string]string)
	for rows.Next() {
		var prompt, reply string
		if err := rows.Scan(&prompt, &reply); err != nil {
			return nil, err
		}
		promptReplies[prompt] = reply
	}
	return promptReplies, nil
}

func (d *Database) GetAllPromptReplies() (map[string]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 强制刷新缓存
	promptReplies, err := d.fetchAllPromptReplies()
	if err != nil {
		return nil, err
	}
	d.promptRepliesCache = promptReplies
	d.promptRepliesCacheTime = time.Now()

	// 返回一个副本
	result := make(map[string]string, len(d.promptRepliesCache))
	for k, v := range d.promptRepliesCache {
		result[k] = v
	}
	return result, nil
}

func (d *Database) invalidateCache(cacheType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch cacheType {
	case "keywords":
		d.keywordsCache = nil
		d.keywordsCacheTime = time.Time{}
	case "whitelist":
		d.whitelistCache = nil
		d.whitelistCacheTime = time.Time{}
	case "promptReplies":
		d.promptRepliesCache = nil
		d.promptRepliesCacheTime = time.Time{}
	default:
		// 清除所有缓存
		d.keywordsCache = nil
		d.whitelistCache = nil
		d.promptRepliesCache = nil
		d.keywordsCacheTime = time.Time{}
		d.whitelistCacheTime = time.Time{}
		d.promptRepliesCacheTime = time.Time{}
	}
}

func (d *Database) Close() error {
	return d.db.Close()
}
