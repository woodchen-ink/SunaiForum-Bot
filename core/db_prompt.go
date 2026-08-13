package core

// prompt_replies 表读写; 存放关键词触发的自动回复。
// 本表不做 TTL 缓存: 运行期查询由 prompt_reply 包的内存映射承担, 这里只在启动加载和管理员增删时被调用。
import (
	"strings"

	"gorm.io/gorm/clause"
)

// AddPromptReply 新增或覆盖一条提示词回复, 提示词统一按小写存储
func (d *Database) AddPromptReply(prompt, reply string) error {
	return d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&PromptReply{
		Prompt: strings.ToLower(prompt),
		Reply:  reply,
	}).Error
}

func (d *Database) DeletePromptReply(prompt string) error {
	return d.db.Where("prompt = ?", strings.ToLower(prompt)).Delete(&PromptReply{}).Error
}

// GetAllPromptReplies 全量读取提示词回复, 返回 提示词 -> 回复 的映射
func (d *Database) GetAllPromptReplies() (map[string]string, error) {
	var rows []PromptReply
	if err := d.db.Find(&rows).Error; err != nil {
		return nil, err
	}

	promptReplies := make(map[string]string, len(rows))
	for _, row := range rows {
		promptReplies[row.Prompt] = row.Reply
	}
	return promptReplies, nil
}
