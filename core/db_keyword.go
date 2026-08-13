package core

// keywords 与 keyword_rejects 表读写。
//
// 词条按 source 区分来源, 这是 AI 自治的安全边界:
//   - manual: 管理员手工维护, AI **只读**, 不得删改
//   - ai:     AI 判定广告后自动提取, AI 可以增删
//
// 否决表的写入是**显式**的, 只在"管理员手动删除 AI 词"和"撤销误判"两处调用 RejectKeyword;
// RemoveKeyword 本身不写否决表 —— 定期整理清掉零命中词只是"这次没用上", 不该永久拉黑。
import (
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 关键词来源
const (
	SourceManual = "manual"
	SourceAI     = "ai"
)

// AddKeyword 新增关键词, 已存在则静默忽略。
// source 为 SourceAI 时先查否决表, 被管理员否决过的词不再添加, 返回 false。
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

	result := d.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Keyword{
		Word:    keyword,
		AddedAt: time.Now(),
		Source:  source,
	})
	if result.Error != nil {
		return false, result.Error
	}
	d.invalidateCache(cacheKeywords)

	return result.RowsAffected > 0, nil
}

// RemoveKeyword 删除关键词, 返回是否删掉了行以及该词原本的来源。
// 来源用于让调用方判断要不要顺手写否决表。
func (d *Database) RemoveKeyword(keyword string) (bool, string, error) {
	var existing Keyword
	err := d.db.Where("keyword = ?", keyword).First(&existing).Error
	if err != nil {
		if isNoRows(err) {
			return false, "", nil
		}
		return false, "", err
	}

	result := d.db.Where("keyword = ?", keyword).Delete(&Keyword{})
	if result.Error != nil {
		return false, "", result.Error
	}
	d.invalidateCache(cacheKeywords)

	return result.RowsAffected > 0, existing.Source, nil
}

// RemoveKeywordsContaining 批量删除包含指定子串的关键词, 返回被删掉的关键词列表
func (d *Database) RemoveKeywordsContaining(substring string) ([]string, error) {
	pattern := "%" + substring + "%"

	var removed []string
	if err := d.db.Model(&Keyword{}).Where("keyword LIKE ?", pattern).Pluck("keyword", &removed).Error; err != nil {
		return nil, err
	}
	if len(removed) == 0 {
		return nil, nil
	}

	if err := d.db.Where("keyword LIKE ?", pattern).Delete(&Keyword{}).Error; err != nil {
		return nil, err
	}

	d.invalidateCache(cacheKeywords)
	return removed, nil
}

// GetActiveKeywords 返回参与匹配的全部关键词 (手工 + AI), 走 TTL 缓存; 增删会立即失效缓存
func (d *Database) GetActiveKeywords() ([]string, error) {
	return d.queryCached(cacheKeywords, func() ([]string, error) {
		var keywords []string
		err := d.db.Model(&Keyword{}).Where("is_auto_added = ?", false).Pluck("keyword", &keywords).Error
		return keywords, err
	})
}

// GetKeywordsBySource 按来源列出关键词及其元数据, 供管理员查看与 AI 定期整理
func (d *Database) GetKeywordsBySource(source string) ([]Keyword, error) {
	var keywords []Keyword
	err := d.db.
		Where("source = ? AND is_auto_added = ?", source, false).
		Order("hit_count DESC, added_at DESC").
		Find(&keywords).Error
	return keywords, err
}

// RecordKeywordHit 累加关键词命中次数; 长期零命中的 AI 词是定期整理时的清理对象
func (d *Database) RecordKeywordHit(keyword string) error {
	return d.db.Model(&Keyword{}).
		Where("keyword = ?", keyword).
		UpdateColumn("hit_count", gorm.Expr("hit_count + 1")).Error
}

// SearchKeywords 模糊查找关键词, 用于删除失败时提示相似项
func (d *Database) SearchKeywords(pattern string) ([]string, error) {
	var keywords []string
	err := d.db.Model(&Keyword{}).Where("keyword LIKE ?", "%"+pattern+"%").Pluck("keyword", &keywords).Error
	return keywords, err
}

func (d *Database) KeywordExists(keyword string) (bool, error) {
	var count int64
	err := d.db.Model(&Keyword{}).Where("keyword = ?", keyword).Count(&count).Error
	return count > 0, err
}

// RejectKeyword 把关键词写入否决表, AI 此后不得再添加它
func (d *Database) RejectKeyword(keyword string) error {
	return d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&KeywordReject{
		Word:       normalizeRejectKey(keyword),
		RejectedAt: time.Now(),
	}).Error
}

func (d *Database) IsKeywordRejected(keyword string) (bool, error) {
	var count int64
	err := d.db.Model(&KeywordReject{}).Where("keyword = ?", normalizeRejectKey(keyword)).Count(&count).Error
	return count > 0, err
}

// normalizeRejectKey 否决表按小写去空白存储, 避免大小写差异导致否决失效
func normalizeRejectKey(keyword string) string {
	return strings.ToLower(strings.TrimSpace(keyword))
}

// CleanupLegacyAutoLinks 排干"同一链接不能发两次"功能遗留的自动添加行。
// 该功能已下线, 不再产生新行, 本函数只负责把历史数据逐步清空。
func (d *Database) CleanupLegacyAutoLinks() (int64, error) {
	twoMonthsAgo := time.Now().AddDate(0, -2, 0)
	result := d.db.
		Where("is_link = ? AND is_auto_added = ? AND added_at < ?", true, true, twoMonthsAgo).
		Delete(&Keyword{})
	if result.Error != nil {
		return 0, result.Error
	}

	if result.RowsAffected > 0 {
		d.invalidateCache(cacheKeywords)
	}
	return result.RowsAffected, nil
}

// CleanupInvalidKeywords 删除内容为空的关键词行并返回删除条数。
//
// 这类行是 2026-08-13 迁移事故的残留: 整表重建把 keyword 列清空, 留下一批 NULL 行。
// 它们永远匹配不到任何消息 (matchKeyword 会跳过空词), 但会让 /list 显示成一串空条目。
// 放在启动流程里自愈, 避免手工去生产库上删数据。
func (d *Database) CleanupInvalidKeywords() (int64, error) {
	result := d.db.Where("keyword IS NULL OR TRIM(keyword) = ''").Delete(&Keyword{})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		d.invalidateCache(cacheKeywords)
	}
	return result.RowsAffected, nil
}
