package core

// user_stats 表读写; 记录每个用户在每个群的发言条数。
// 用途是识别"新用户": AI 审核只对新用户的前若干条消息全量开启, 老用户靠弱信号触发, 以此控制调用量。
import "time"

// BumpUserMessageCount 累加发言计数并返回累加后的条数
func (d *Database) BumpUserMessageCount(userID, chatID int64) (int, error) {
	_, err := d.db.Exec(`
		INSERT INTO user_stats (user_id, chat_id, message_count, first_seen_at, last_seen_at)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(user_id, chat_id) DO UPDATE SET
			message_count = message_count + 1,
			last_seen_at = excluded.last_seen_at`,
		userID, chatID, time.Now(), time.Now())
	if err != nil {
		return 0, err
	}

	var count int
	err = d.db.QueryRow(
		"SELECT message_count FROM user_stats WHERE user_id = ? AND chat_id = ?", userID, chatID).Scan(&count)
	if err != nil {
		if isNoRows(err) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// CleanupStaleUserStats 清理长期不发言用户的统计, 避免表无限增长。
// 注意副作用: 被清理的用户再次发言会重新被当作新用户走 AI 审核, 这正是期望行为。
func (d *Database) CleanupStaleUserStats(olderThan time.Duration) (int64, error) {
	result, err := d.db.Exec("DELETE FROM user_stats WHERE last_seen_at < ?", time.Now().Add(-olderThan))
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
