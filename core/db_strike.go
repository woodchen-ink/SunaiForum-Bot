package core

// user_strikes 表读写; 记录每个用户在每个群的违规累计次数, 达到阈值触发自动封禁。
// 计分入库而非放内存, 是为了让容器重启不清零 —— 封禁是重动作, 不能因为一次重启就放过惯犯。
import "time"

// AddStrike 给用户记一次违规并返回累计次数
func (d *Database) AddStrike(userID, chatID int64) (int, error) {
	_, err := d.db.Exec(`
		INSERT INTO user_strikes (user_id, chat_id, strikes, last_hit_at) VALUES (?, ?, 1, ?)
		ON CONFLICT(user_id, chat_id) DO UPDATE SET strikes = strikes + 1, last_hit_at = excluded.last_hit_at`,
		userID, chatID, time.Now())
	if err != nil {
		return 0, err
	}
	return d.GetStrikes(userID, chatID)
}

// GetStrikes 读取累计违规次数, 无记录返回 0
func (d *Database) GetStrikes(userID, chatID int64) (int, error) {
	var strikes int
	err := d.db.QueryRow(
		"SELECT strikes FROM user_strikes WHERE user_id = ? AND chat_id = ?", userID, chatID).Scan(&strikes)
	if err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, err
	}
	return strikes, nil
}

// ResetStrikes 清空某用户的违规计分, 供管理员解封后调用
func (d *Database) ResetStrikes(userID, chatID int64) error {
	_, err := d.db.Exec("DELETE FROM user_strikes WHERE user_id = ? AND chat_id = ?", userID, chatID)
	return err
}

// CleanupStaleStrikes 删除长期无新违规的计分记录, 避免表无限增长
func (d *Database) CleanupStaleStrikes(olderThan time.Duration) (int64, error) {
	result, err := d.db.Exec("DELETE FROM user_strikes WHERE last_hit_at < ?", time.Now().Add(-olderThan))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
