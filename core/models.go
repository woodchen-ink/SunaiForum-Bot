package core

// GORM 数据模型。
//
// 这些表在迁移到 GORM 之前是手写 SQL 建的, 生产库里已有数据。因此:
//   - 每个字段都显式声明 gorm:"column:..." , 不依赖 GORM 的自动 snake_case 推导
//   - 每个模型都实现 TableName(), 不依赖 GORM 的自动复数化 (否则 whitelist 会变成 whitelists)
//   - 索引显式命名且避开老库里已有的索引名, 防止 AutoMigrate 撞名失败
//
// 改动任何字段前先想清楚 AutoMigrate 会在存量库上做什么: 它只增不减,
// 加列安全, 但改类型/加约束在 SQLite 上会触发整表重建。
import "time"

// Keyword 过滤关键词。
// IsLink / IsAutoAdded 是"同一链接不能发两次"功能的遗留字段, 已不再写入,
// 保留是为了让 CleanupLegacyAutoLinks 能排干历史数据。
type Keyword struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Word        string    `gorm:"column:keyword;uniqueIndex:uq_keywords_word"`
	IsLink      bool      `gorm:"column:is_link;default:false"`
	IsAutoAdded bool      `gorm:"column:is_auto_added;default:false"`
	AddedAt     time.Time `gorm:"column:added_at"`
	Source      string    `gorm:"column:source;not null;default:manual"`
	HitCount    int       `gorm:"column:hit_count;not null;default:0"`
}

func (Keyword) TableName() string { return "keywords" }

// KeywordReject 被管理员否决的关键词, AI 不得再次添加
type KeywordReject struct {
	Word       string    `gorm:"column:keyword;primaryKey"`
	RejectedAt time.Time `gorm:"column:rejected_at"`
}

func (KeywordReject) TableName() string { return "keyword_rejects" }

// PromptReply 关键词触发的自动回复; Prompt 统一按小写存储
type PromptReply struct {
	Prompt string `gorm:"column:prompt;primaryKey"`
	Reply  string `gorm:"column:reply;not null"`
}

func (PromptReply) TableName() string { return "prompt_replies" }

// Config 需要跨容器重启保留的运行时状态
type Config struct {
	Key   string `gorm:"column:key;primaryKey"`
	Value string `gorm:"column:value"`
}

func (Config) TableName() string { return "config" }

// UserStat 每个用户在每个群的发言统计, 用于识别"新用户"决定是否走 AI 审核
type UserStat struct {
	UserID       int64     `gorm:"column:user_id;primaryKey"`
	ChatID       int64     `gorm:"column:chat_id;primaryKey"`
	MessageCount int       `gorm:"column:message_count;not null;default:0"`
	FirstSeenAt  time.Time `gorm:"column:first_seen_at"`
	LastSeenAt   time.Time `gorm:"column:last_seen_at"`
}

func (UserStat) TableName() string { return "user_stats" }

// UserStrike 违规累计计分; 落库而非放内存, 避免重启放过惯犯
type UserStrike struct {
	UserID    int64     `gorm:"column:user_id;primaryKey"`
	ChatID    int64     `gorm:"column:chat_id;primaryKey"`
	Strikes   int       `gorm:"column:strikes;not null;default:0"`
	LastHitAt time.Time `gorm:"column:last_hit_at"`
}

func (UserStrike) TableName() string { return "user_strikes" }

// ModerationActionRow 一次审核处置的落库形态。
// LearnedWords 以换行分隔存成单列: 这张表只按主键查, 不需要对词做关联查询,
// 建关联表反而让"撤销"这一个用例多出一次 join。
type ModerationActionRow struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID       int64     `gorm:"column:user_id;not null"`
	ChatID       int64     `gorm:"column:chat_id;not null"`
	UserName     string    `gorm:"column:user_name"`
	MessageText  string    `gorm:"column:message_text"`
	Rule         string    `gorm:"column:rule"`
	LearnedWords string    `gorm:"column:learned_words"`
	Banned       bool      `gorm:"column:banned;not null;default:false"`
	Undone       bool      `gorm:"column:undone;not null;default:false"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (ModerationActionRow) TableName() string { return "moderation_actions" }

// allModels AutoMigrate 的目标清单; 新增表必须登记在这里
func allModels() []any {
	return []any{
		&Keyword{},
		&KeywordReject{},
		&PromptReply{},
		&Config{},
		&UserStat{},
		&UserStrike{},
		&ModerationActionRow{},
	}
}
