package core

// user_stats 表读写; 记录每个用户在每个群的发言条数。
// 用途是识别"新用户": AI 审核只对新用户的前若干条消息全量开启, 老用户靠弱信号触发, 以此控制调用量。
import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BumpUserMessageCount 累加发言计数并返回累加后的条数
func (d *Database) BumpUserMessageCount(userID, chatID int64) (int, error) {
	now := time.Now()
	err := d.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"message_count": gorm.Expr("user_stats.message_count + 1"),
			"last_seen_at":  now,
		}),
	}).Create(&UserStat{
		UserID:       userID,
		ChatID:       chatID,
		MessageCount: 1,
		FirstSeenAt:  now,
		LastSeenAt:   now,
	}).Error
	if err != nil {
		return 0, err
	}

	var row UserStat
	if err := d.db.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&row).Error; err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, err
	}
	return row.MessageCount, nil
}

// CleanupStaleUserStats 清理长期不发言用户的统计, 避免表无限增长。
// 注意副作用: 被清理的用户再次发言会重新被当作新用户走 AI 审核, 这正是期望行为。
func (d *Database) CleanupStaleUserStats(olderThan time.Duration) (int64, error) {
	result := d.db.Where("last_seen_at < ?", time.Now().Add(-olderThan)).Delete(&UserStat{})
	return result.RowsAffected, result.Error
}
