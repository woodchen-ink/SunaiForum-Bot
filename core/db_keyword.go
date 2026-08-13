package core

// keywords 表读写。
// is_link / is_auto_added 两列是"同一链接不能发两次"功能的遗留字段, 该功能已下线,
// 新写入的行两列恒为 false; 保留列是为了让 CleanupLegacyAutoLinks 能排干历史数据。
import "time"

// AddKeyword 新增一个手动维护的过滤关键词, 已存在则静默忽略
func (d *Database) AddKeyword(keyword string) error {
	_, err := d.db.Exec(
		"INSERT OR IGNORE INTO keywords (keyword, is_link, is_auto_added, added_at) VALUES (?, FALSE, FALSE, ?)",
		keyword, time.Now())
	if err != nil {
		return err
	}
	d.invalidateCache(cacheKeywords)
	return nil
}

// RemoveKeyword 删除指定关键词, 返回是否真的删掉了行
func (d *Database) RemoveKeyword(keyword string) (bool, error) {
	result, err := d.db.Exec("DELETE FROM keywords WHERE keyword = ?", keyword)
	if err != nil {
		return false, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	d.invalidateCache(cacheKeywords)
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

// GetAllManualKeywords 返回全部手动维护的关键词, 走 TTL 缓存; 增删关键词会立即失效缓存
func (d *Database) GetAllManualKeywords() ([]string, error) {
	return d.queryCached(cacheKeywords, "SELECT keyword FROM keywords WHERE is_auto_added = ?", false)
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
