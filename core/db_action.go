package core

// moderation_actions 表读写; 记录每一次审核处置, 用于管理员一键撤销。
//
// 落库的原因: 撤销按钮的 callback_data 上限 64 字节, 装不下原文和上下文,
// 只能存一个自增 id, 详情回表查。同时这张表也是审核行为的审计记录。
import (
	"strings"
	"time"
)

// ModerationAction 一次审核处置的完整上下文 (对外形态, LearnedWords 已拆成切片)
type ModerationAction struct {
	ID           int64
	UserID       int64
	ChatID       int64
	UserName     string
	MessageText  string
	Rule         string
	LearnedWords []string // 本次判定新增的 AI 关键词, 撤销时一并回滚
	Banned       bool
	Undone       bool
	CreatedAt    time.Time
}

// RecordModerationAction 记录一次处置并返回其 id
func (d *Database) RecordModerationAction(action ModerationAction) (int64, error) {
	row := ModerationActionRow{
		UserID:       action.UserID,
		ChatID:       action.ChatID,
		UserName:     action.UserName,
		MessageText:  action.MessageText,
		Rule:         action.Rule,
		LearnedWords: strings.Join(action.LearnedWords, "\n"),
		Banned:       action.Banned,
		CreatedAt:    time.Now(),
	}
	if err := d.db.Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// GetModerationAction 按 id 取回处置详情
func (d *Database) GetModerationAction(id int64) (ModerationAction, error) {
	var row ModerationActionRow
	if err := d.db.First(&row, id).Error; err != nil {
		return ModerationAction{}, err
	}

	action := ModerationAction{
		ID:          row.ID,
		UserID:      row.UserID,
		ChatID:      row.ChatID,
		UserName:    row.UserName,
		MessageText: row.MessageText,
		Rule:        row.Rule,
		Banned:      row.Banned,
		Undone:      row.Undone,
		CreatedAt:   row.CreatedAt,
	}
	if row.LearnedWords != "" {
		action.LearnedWords = strings.Split(row.LearnedWords, "\n")
	}
	return action, nil
}

// MarkActionUndone 把处置标记为已撤销; 返回 false 表示此前已经撤销过。
// 用条件更新实现幂等 —— 管理员连点两下按钮不该反复解封、反复扣分。
func (d *Database) MarkActionUndone(id int64) (bool, error) {
	result := d.db.Model(&ModerationActionRow{}).
		Where("id = ? AND undone = ?", id, false).
		Update("undone", true)
	return result.RowsAffected > 0, result.Error
}

// CleanupOldActions 清理陈旧的处置记录
func (d *Database) CleanupOldActions(olderThan time.Duration) (int64, error) {
	result := d.db.Where("created_at < ?", time.Now().Add(-olderThan)).Delete(&ModerationActionRow{})
	return result.RowsAffected, result.Error
}
