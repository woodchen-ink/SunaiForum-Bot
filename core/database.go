package core

// 数据库连接、建表与列表缓存基础设施; 各张表的具体读写拆在 db_*.go
import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// cacheTTL 列表缓存存活时间, 过期后回源查库; 写操作会主动失效对应缓存, 因此改动即时生效
const cacheTTL = 5 * time.Minute

// cacheKind 标识一份列表缓存, 用常量代替字符串避免拼写错误
type cacheKind int

const (
	cacheKeywords cacheKind = iota
	cacheWhitelist
)

// cachedList 带加载时间的字符串列表缓存, 零值表示尚未加载
type cachedList struct {
	items    []string
	loadedAt time.Time
}

// expired 判断缓存未加载或已超过 TTL
func (c *cachedList) expired() bool {
	return c.items == nil || time.Since(c.loadedAt) > cacheTTL
}

type Database struct {
	db *sql.DB

	// mu 保护下面所有缓存字段
	mu             sync.Mutex
	manualKeywords cachedList // 手动维护的过滤关键词, 每条群消息都要读
	whitelist      cachedList // 白名单域名
}

// NewDatabase 打开 SQLite 连接并确保所有表存在
func NewDatabase() (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(DBFile), os.ModePerm); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", DBFile)
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}
	if err := database.createTables(); err != nil {
		return nil, err
	}

	return database, nil
}

// createTables 幂等建表, 每次启动都会执行
func (d *Database) createTables() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS keywords (
					id INTEGER PRIMARY KEY,
					keyword TEXT UNIQUE,
					is_link BOOLEAN DEFAULT FALSE,
					is_auto_added BOOLEAN DEFAULT FALSE,
					added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword ON keywords(keyword)`,
		`CREATE INDEX IF NOT EXISTS idx_added_at ON keywords(added_at)`,
		`CREATE TABLE IF NOT EXISTS whitelist (
					id INTEGER PRIMARY KEY,
					domain TEXT UNIQUE
			)`,
		`CREATE INDEX IF NOT EXISTS idx_domain ON whitelist(domain)`,
		`CREATE TABLE IF NOT EXISTS prompt_replies (
					prompt TEXT PRIMARY KEY,
					reply TEXT NOT NULL
			)`,
		`CREATE TABLE IF NOT EXISTS config (
					key TEXT PRIMARY KEY,
					value TEXT
			)`,
		`CREATE TABLE IF NOT EXISTS keyword_rejects (
					keyword TEXT PRIMARY KEY,
					rejected_at TIMESTAMP
			)`,
		`CREATE TABLE IF NOT EXISTS moderation_actions (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					user_id INTEGER NOT NULL,
					chat_id INTEGER NOT NULL,
					user_name TEXT,
					message_text TEXT,
					rule TEXT,
					learned_words TEXT,
					banned INTEGER NOT NULL DEFAULT 0,
					undone INTEGER NOT NULL DEFAULT 0,
					created_at TIMESTAMP
			)`,
		`CREATE TABLE IF NOT EXISTS user_stats (
					user_id INTEGER NOT NULL,
					chat_id INTEGER NOT NULL,
					message_count INTEGER NOT NULL DEFAULT 0,
					first_seen_at TIMESTAMP,
					last_seen_at TIMESTAMP,
					PRIMARY KEY (user_id, chat_id)
			)`,
		`CREATE TABLE IF NOT EXISTS user_strikes (
					user_id INTEGER NOT NULL,
					chat_id INTEGER NOT NULL,
					strikes INTEGER NOT NULL DEFAULT 0,
					last_hit_at TIMESTAMP,
					PRIMARY KEY (user_id, chat_id)
			)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("执行建表语句失败 '%s': %w", query, err)
		}
	}

	// 增量列: 老库里 keywords 表没有这两列, 需要在原表上补齐
	migrations := []struct{ table, column, ddl string }{
		{"keywords", "source", "TEXT NOT NULL DEFAULT 'manual'"},
		{"keywords", "hit_count", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, m := range migrations {
		if err := d.addColumnIfMissing(m.table, m.column, m.ddl); err != nil {
			return err
		}
	}

	log.Println("[Database] 所有必要的表都已创建")
	return nil
}

// addColumnIfMissing 幂等地给已有表补一列; SQLite 不支持 ADD COLUMN IF NOT EXISTS, 只能先查再加
func (d *Database) addColumnIfMissing(table, column, ddl string) error {
	rows, err := d.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("读取表 %s 结构失败: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid, notNull, pk int
			name, colType    string
			defaultValue     any
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("解析表 %s 结构失败: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if _, err := d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl)); err != nil {
		return fmt.Errorf("为表 %s 添加列 %s 失败: %w", table, column, err)
	}
	log.Printf("[Database] 已为表 %s 添加列 %s", table, column)
	return nil
}

// executeQuery 执行单列查询并收集为字符串切片
func (d *Database) executeQuery(query string, args ...any) ([]string, error) {
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

	return results, rows.Err()
}

// queryCached 读取带 TTL 的列表缓存, 未命中时回源查库。
// 返回的是副本, 调用方修改不会污染缓存。
func (d *Database) queryCached(kind cacheKind, query string, args ...any) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cache := d.cacheOf(kind)
	if cache.expired() {
		items, err := d.executeQuery(query, args...)
		if err != nil {
			return nil, err
		}
		cache.items = items
		cache.loadedAt = time.Now()
	}

	result := make([]string, len(cache.items))
	copy(result, cache.items)
	return result, nil
}

// invalidateCache 清空指定列表缓存, 由写操作在提交后调用
func (d *Database) invalidateCache(kind cacheKind) {
	d.mu.Lock()
	defer d.mu.Unlock()
	*d.cacheOf(kind) = cachedList{}
}

// cacheOf 取指定种类的缓存指针; 调用方必须已持有 d.mu
func (d *Database) cacheOf(kind cacheKind) *cachedList {
	if kind == cacheWhitelist {
		return &d.whitelist
	}
	return &d.manualKeywords
}

// CountRecords 统计指定表的记录数; 表名走白名单校验, 防止拼接注入
func (d *Database) CountRecords(tableName string) (int, error) {
	allowedTables := map[string]bool{
		"keywords":       true,
		"whitelist":      true,
		"prompt_replies": true,
		"config":         true,
	}

	if !allowedTables[tableName] {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	if err := d.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("统计表 %s 记录数失败: %w", tableName, err)
	}
	return count, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// isNoRows 判断错误是否为"查询无结果", 供各 db_*.go 把空结果转成零值
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
