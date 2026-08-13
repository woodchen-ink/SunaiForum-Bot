package core

// user_strikes 表读写; 记录每个用户在每个群的违规累计次数, 达到阈值触发自动封禁。
// 计分入库而非放内存, 是为了让容器重启不清零 —— 封禁是重动作, 不能因为一次重启就放过惯犯。
import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AddStrike 给用户记一次违规并返回累计次数
func (d *Database) AddStrike(userID, chatID int64) (int, error) {
	err := d.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "chat_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"strikes":     gorm.Expr("user_strikes.strikes + 1"),
			"last_hit_at": time.Now(),
		}),
	}).Create(&UserStrike{
		UserID:    userID,
		ChatID:    chatID,
		Strikes:   1,
		LastHitAt: time.Now(),
	}).Error
	if err != nil {
		return 0, err
	}
	return d.GetStrikes(userID, chatID)
}

// GetStrikes 读取累计违规次数, 无记录返回 0
func (d *Database) GetStrikes(userID, chatID int64) (int, error) {
	var row UserStrike
	err := d.db.Where("user_id = ? AND chat_id = ?", userID, chatID).First(&row).Error
	if err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, err
	}
	return row.Strikes, nil
}

// DecrementStrike 撤销一次违规计分, 不会减到负数
func (d *Database) DecrementStrike(userID, chatID int64) error {
	return d.db.Model(&UserStrike{}).
		Where("user_id = ? AND chat_id = ?", userID, chatID).
		UpdateColumn("strikes", gorm.Expr("MAX(strikes - 1, 0)")).Error
}

// ResetStrikes 清空某用户的违规计分, 供管理员解封后调用
func (d *Database) ResetStrikes(userID, chatID int64) error {
	return d.db.Where("user_id = ? AND chat_id = ?", userID, chatID).Delete(&UserStrike{}).Error
}

// CleanupStaleStrikes 删除长期无新违规的计分记录, 避免表无限增长
func (d *Database) CleanupStaleStrikes(olderThan time.Duration) (int64, error) {
	result := d.db.Where("last_hit_at < ?", time.Now().Add(-olderThan)).Delete(&UserStrike{})
	return result.RowsAffected, result.Error
}
