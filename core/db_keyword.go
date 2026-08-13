package core

// keywords 表读写。
//
// 词条按 source 区分来源, 这是 AI 自治的安全边界:
//   - manual: 管理员手工维护, AI **只读**, 不得删改
//   - ai:     AI 判定广告后自动提取, AI 可以增删, 属于它的自治范围
//
// 管理员删除一条 ai 词, 该词会进入 keyword_rejects 否决表, AI 不得再次添加 ——
// 删除动作本身就是否决信号, 不需要额外命令。
//
// is_link / is_auto_added 是"同一链接不能发两次"功能的遗留列, 已不再写入, 保留仅为排干历史数据。
import (
	"fmt"
	"strings"
	"time"
)

// 关键词来源
const (
	SourceManual = "manual"
	SourceAI     = "ai"
)

// Keyword 一条过滤关键词及其元数据
type Keyword struct {
	Word     string
	Source   string
	HitCount int
	AddedAt  time.Time
}

// AddKeyword 新增关键词, 已存在则静默忽略。
// source 为 SourceAI 时会先查否决表, 被管理员否决过的词不再添加, 返回 false。
func (d *Database) AddKeyword(keyword, source string) (bool, error) {
	if source == SourceAI {
		rejected, err := d.IsKeywordRejected(keyword)
		if err != nil {
			return false, err
		}
		if rejected {
			return false, nil
		}
	}

	result, err := d.db.Exec(
		`INSERT OR IGNORE INTO keywords (keyword, is_link, is_auto_added, added_at, source, hit_count)
		 VALUES (?, FALSE, FALSE, ?, ?, 0)`,
		keyword, time.Now(), source)
	if err != nil {
		return false, err
	}
	d.invalidateCache(cacheKeywords)

	rows, err := result.RowsAffected()
	return rows > 0, err
}

// RemoveKeyword 删除关键词。删除的若是 AI 添加的词, 同时写入否决表阻止 AI 再加回来。
func (d *Database) RemoveKeyword(keyword string) (bool, error) {
	var source string
	err := d.db.QueryRow("SELECT source FROM keywords WHERE keyword = ?", keyword).Scan(&source)
	if err != nil && !isNoRows(err) {
		return false, err
	}

	result, err := d.db.Exec("DELETE FROM keywords WHERE keyword = ?", keyword)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	d.invalidateCache(cacheKeywords)

	if rowsAffected > 0 && source == SourceAI {
		if err := d.RejectKeyword(keyword); err != nil {
			return true, fmt.Errorf("已删除关键词但写入否决表失败: %w", err)
		}
	}
	return rowsAffected > 0, nil
}

// RemoveKeywordsContaining 批量删除包含指定子串的关键词, 返回被删掉的关键词列表
func (d *Database) RemoveKeywordsContaining(substring string) ([]string, error) {
	pattern := "%" + substring + "%"

	removedKeywords, err := d.executeQuery("SELECT keyword FROM keywords WHERE keyword LIKE ?", pattern)
	if err != nil {
		return nil, err
	}

	if _, err := d.db.Exec("DELETE FROM keywords WHERE keyword LIKE ?", pattern); err != nil {
		return nil, err
	}

	d.invalidateCache(cacheKeywords)
	return removedKeywords, nil
}

// GetActiveKeywords 返回参与匹配的全部关键词 (手工 + AI), 走 TTL 缓存; 增删会立即失效缓存
func (d *Database) GetActiveKeywords() ([]string, error) {
	return d.queryCached(cacheKeywords, "SELECT keyword FROM keywords WHERE is_auto_added = ?", false)
}

// GetKeywordsBySource 按来源列出关键词及其元数据, 供管理员查看与 AI 定期整理
func (d *Database) GetKeywordsBySource(source string) ([]Keyword, error) {
	rows, err := d.db.Query(
		`SELECT keyword, source, hit_count, added_at FROM keywords
		 WHERE source = ? AND is_auto_added = FALSE ORDER BY hit_count DESC, added_at DESC`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keywords []Keyword
	for rows.Next() {
		var k Keyword
		if err := rows.Scan(&k.Word, &k.Source, &k.HitCount, &k.AddedAt); err != nil {
			return nil, err
		}
		keywords = append(keywords, k)
	}
	return keywords, rows.Err()
}

// RecordKeywordHit 累加关键词命中次数; 长期零命中的 AI 词是定期整理时的清理对象
func (d *Database) RecordKeywordHit(keyword string) error {
	_, err := d.db.Exec("UPDATE keywords SET hit_count = hit_count + 1 WHERE keyword = ?", keyword)
	return err
}

// SearchKeywords 模糊查找关键词, 用于删除失败时提示相似项
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

// RejectKeyword 把关键词写入否决表, AI 此后不得再添加它
func (d *Database) RejectKeyword(keyword string) error {
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO keyword_rejects (keyword, rejected_at) VALUES (?, ?)",
		strings.ToLower(strings.TrimSpace(keyword)), time.Now())
	return err
}

func (d *Database) IsKeywordRejected(keyword string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM keyword_rejects WHERE keyword = ?",
		strings.ToLower(strings.TrimSpace(keyword))).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CleanupLegacyAutoLinks 排干"同一链接不能发两次"功能遗留的自动添加行。
// 该功能已下线, 不再产生新行, 本函数只负责把历史数据逐步清空。
func (d *Database) CleanupLegacyAutoLinks() (int64, error) {
	twoMonthsAgo := time.Now().AddDate(0, -2, 0)
	result, err := d.db.Exec(
		"DELETE FROM keywords WHERE is_link = TRUE AND is_auto_added = TRUE AND added_at < ?", twoMonthsAgo)
	if err != nil {
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}

	d.invalidateCache(cacheKeywords)
	return rowsAffected, nil
}
