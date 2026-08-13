package core

// prompt_replies 表读写; 存放关键词触发的自动回复。
// 本表不做 TTL 缓存: 运行期查询由 prompt_reply 包的内存映射承担, 这里只在启动加载和管理员查看时被调用。
import "strings"

// AddPromptReply 新增或覆盖一条提示词回复, 提示词统一按小写存储
func (d *Database) AddPromptReply(prompt, reply string) error {
	_, err := d.db.Exec(
		"INSERT OR REPLACE INTO prompt_replies (prompt, reply) VALUES (?, ?)",
		strings.ToLower(prompt), reply)
	return err
}

func (d *Database) DeletePromptReply(prompt string) error {
	_, err := d.db.Exec("DELETE FROM prompt_replies WHERE prompt = ?", strings.ToLower(prompt))
	return err
}

// GetAllPromptReplies 全量读取提示词回复, 返回 提示词 -> 回复 的映射
func (d *Database) GetAllPromptReplies() (map[string]string, error) {
	rows, err := d.db.Query("SELECT prompt, reply FROM prompt_replies")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	promptReplies := make(map[string]string)
	for rows.Next() {
		var prompt, reply string
		if err := rows.Scan(&prompt, &reply); err != nil {
			return nil, err
		}
		promptReplies[prompt] = reply
	}
	return promptReplies, rows.Err()
}
