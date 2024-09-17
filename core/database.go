package core

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	db             *sql.DB
	dbFile         string
	keywordsCache  []string
	whitelistCache []string
	cacheTime      time.Time
	mu             sync.Mutex
}

func NewDatabase(dbFile string) (*Database, error) {
	os.MkdirAll(filepath.Dir(dbFile), os.ModePerm)
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return nil, err
	}

	database := &Database{
		db:     db,
		dbFile: dbFile,
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
	d.invalidateCache()
	return nil
}

func (d *Database) RemoveKeyword(keyword string) error {
	_, err := d.db.Exec("DELETE FROM keywords WHERE keyword = ?", keyword)
	if err != nil {
		return err
	}
	d.invalidateCache()
	return nil
}

func (d *Database) GetAllKeywords() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.keywordsCache == nil || time.Since(d.cacheTime) > 5*time.Minute {
		keywords, err := d.executeQuery("SELECT keyword FROM keywords")
		if err != nil {
			return nil, err
		}
		d.keywordsCache = keywords
		d.cacheTime = time.Now()
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

	d.invalidateCache()
	return removedKeywords, nil
}

func (d *Database) AddWhitelist(domain string) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO whitelist (domain) VALUES (?)", domain)
	if err != nil {
		return err
	}
	d.invalidateCache()
	return nil
}

func (d *Database) RemoveWhitelist(domain string) error {
	_, err := d.db.Exec("DELETE FROM whitelist WHERE domain = ?", domain)
	if err != nil {
		return err
	}
	d.invalidateCache()
	return nil
}

func (d *Database) GetAllWhitelist() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.whitelistCache == nil || time.Since(d.cacheTime) > 5*time.Minute {
		whitelist, err := d.executeQuery("SELECT domain FROM whitelist")
		if err != nil {
			return nil, err
		}
		d.whitelistCache = whitelist
		d.cacheTime = time.Now()
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

func (d *Database) invalidateCache() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.keywordsCache = nil
	d.whitelistCache = nil
	d.cacheTime = time.Time{}
}

func (d *Database) Close() error {
	return d.db.Close()
}
