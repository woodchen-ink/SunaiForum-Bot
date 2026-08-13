package core

// 数据库连接、迁移与列表缓存基础设施; 各张表的具体读写拆在 db_*.go
//
// 驱动用 glebarez/sqlite (纯 Go, 内部走 modernc.org/sqlite) 而不是 gorm.io/driver/sqlite,
// 后者依赖 CGO, 会破坏 CGO_ENABLED=0 的 amd64/arm64 交叉编译。
import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// cacheTTL 列表缓存存活时间, 过期后回源查库; 写操作会主动失效对应缓存, 因此改动即时生效
const cacheTTL = 5 * time.Minute

// cacheKind 标识一份列表缓存, 用常量代替字符串避免拼写错误
type cacheKind int

const (
	cacheKeywords cacheKind = iota
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
	db *gorm.DB

	// mu 保护下面所有缓存字段
	mu             sync.Mutex
	manualKeywords cachedList // 参与匹配的关键词, 每条群消息都要读
}

// NewDatabase 打开 SQLite 连接并把 schema 迁移到最新
func NewDatabase() (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(DBFile), os.ModePerm); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	gormLogLevel := logger.Warn
	if DebugMode {
		gormLogLevel = logger.Info
	}

	db, err := gorm.Open(sqlite.Open(DBFile), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
		// 表名已由各模型的 TableName() 显式指定, 关掉复数化避免歧义
		NamingStrategy: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		return nil, err
	}

	return database, nil
}

// NewDatabaseAt 在指定路径打开数据库, 供测试使用, 不触碰全局 DBFile
func NewDatabaseAt(path string) (*Database, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		return nil, err
	}
	return database, nil
}

// migrate 把 schema 对齐到 models.go 的定义。
// AutoMigrate 只增不减: 会补齐缺失的表、列和索引, 不会删除或重命名已有的东西,
// 因此在存量库上是安全的 —— 这一点由 database_test.go 用真实老 schema 验证。
func (d *Database) migrate() error {
	if err := d.db.AutoMigrate(allModels()...); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	log.Println("[Database] schema 已对齐")
	return nil
}

// queryCached 读取带 TTL 的列表缓存, 未命中时回源查库。
// 返回的是副本, 调用方修改不会污染缓存。
func (d *Database) queryCached(kind cacheKind, load func() ([]string, error)) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cache := d.cacheOf(kind)
	if cache.expired() {
		items, err := load()
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
func (d *Database) cacheOf(cacheKind) *cachedList {
	return &d.manualKeywords
}

// CountRecords 统计指定模型的记录数
func (d *Database) CountRecords(model any) (int64, error) {
	var count int64
	if err := d.db.Model(model).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计记录数失败: %w", err)
	}
	return count, nil
}

func (d *Database) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// isNoRows 判断错误是否为"查询无结果", 供各 db_*.go 把空结果转成零值
func isNoRows(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
