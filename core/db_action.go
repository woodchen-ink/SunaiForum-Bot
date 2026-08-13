package core

// moderation_actions 表读写; 记录每一次审核处置, 用于管理员一键撤销。
//
// 落库的原因: 撤销按钮的 callback_data 上限 64 字节, 装不下原文和上下文,
// 只能存一个自增 id, 详情回表查。同时这张表也是审核行为的审计记录。
import (
	"strings"
	"time"
)

// ModerationAction 一次审核处置的完整上下文
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
	result, err := d.db.Exec(`
		INSERT INTO moderation_actions
			(user_id, chat_id, user_name, message_text, rule, learned_words, banned, undone, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		action.UserID, action.ChatID, action.UserName, action.MessageText, action.Rule,
		strings.Join(action.LearnedWords, "\n"), boolToInt(action.Banned), time.Now())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// GetModerationAction 按 id 取回处置详情
func (d *Database) GetModerationAction(id int64) (ModerationAction, error) {
	var (
		action       ModerationAction
		learnedWords string
		banned       int
		undone       int
	)

	err := d.db.QueryRow(`
		SELECT id, user_id, chat_id, user_name, message_text, rule, learned_words, banned, undone, created_at
		FROM moderation_actions WHERE id = ?`, id).
		Scan(&action.ID, &action.UserID, &action.ChatID, &action.UserName, &action.MessageText,
			&action.Rule, &learnedWords, &banned, &undone, &action.CreatedAt)
	if err != nil {
		return ModerationAction{}, err
	}

	action.Banned = banned != 0
	action.Undone = undone != 0
	if learnedWords != "" {
		action.LearnedWords = strings.Split(learnedWords, "\n")
	}
	return action, nil
}

// MarkActionUndone 把处置标记为已撤销; 返回 false 表示此前已经撤销过, 调用方应避免重复回滚
func (d *Database) MarkActionUndone(id int64) (bool, error) {
	result, err := d.db.Exec("UPDATE moderation_actions SET undone = 1 WHERE id = ? AND undone = 0", id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// CleanupOldActions 清理陈旧的处置记录
func (d *Database) CleanupOldActions(olderThan time.Duration) (int64, error) {
	result, err := d.db.Exec("DELETE FROM moderation_actions WHERE created_at < ?", time.Now().Add(-olderThan))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DecrementStrike 撤销一次违规计分, 不会减到负数
func (d *Database) DecrementStrike(userID, chatID int64) error {
	_, err := d.db.Exec(
		"UPDATE user_strikes SET strikes = MAX(strikes - 1, 0) WHERE user_id = ? AND chat_id = ?",
		userID, chatID)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
