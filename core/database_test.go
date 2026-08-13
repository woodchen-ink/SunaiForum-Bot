package core

// 迁移安全性验证。
//
// 生产库是手写 SQL 建的, 迁到 GORM 后 AutoMigrate 会在存量表上跑。这里用**迁移前的真实 schema**
// 造一个带数据的库, 再跑 AutoMigrate, 确认: 数据没丢、没多出重复列、读写照常。
// 这是这次迁移唯一能在本地做的实质性验证, 改动 models.go 后必须重跑。
import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/glebarez/go-sqlite"
)

// legacySchema 迁移到 GORM 之前, 由 createTables() 手写建出来的表结构
var legacySchema = []string{
	`CREATE TABLE IF NOT EXISTS keywords (
		id INTEGER PRIMARY KEY,
		keyword TEXT UNIQUE,
		is_link BOOLEAN DEFAULT FALSE,
		is_auto_added BOOLEAN DEFAULT FALSE,
		added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		source TEXT NOT NULL DEFAULT 'manual',
		hit_count INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_keyword ON keywords(keyword)`,
	`CREATE INDEX IF NOT EXISTS idx_added_at ON keywords(added_at)`,
	`CREATE TABLE IF NOT EXISTS whitelist (id INTEGER PRIMARY KEY, domain TEXT UNIQUE)`,
	`CREATE INDEX IF NOT EXISTS idx_domain ON whitelist(domain)`,
	`CREATE TABLE IF NOT EXISTS prompt_replies (prompt TEXT PRIMARY KEY, reply TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT)`,
	`CREATE TABLE IF NOT EXISTS keyword_rejects (keyword TEXT PRIMARY KEY, rejected_at TIMESTAMP)`,
	`CREATE TABLE IF NOT EXISTS user_strikes (
		user_id INTEGER NOT NULL, chat_id INTEGER NOT NULL,
		strikes INTEGER NOT NULL DEFAULT 0, last_hit_at TIMESTAMP,
		PRIMARY KEY (user_id, chat_id)
	)`,
	`CREATE TABLE IF NOT EXISTS user_stats (
		user_id INTEGER NOT NULL, chat_id INTEGER NOT NULL,
		message_count INTEGER NOT NULL DEFAULT 0,
		first_seen_at TIMESTAMP, last_seen_at TIMESTAMP,
		PRIMARY KEY (user_id, chat_id)
	)`,
	`CREATE TABLE IF NOT EXISTS moderation_actions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL, chat_id INTEGER NOT NULL,
		user_name TEXT, message_text TEXT, rule TEXT, learned_words TEXT,
		banned INTEGER NOT NULL DEFAULT 0, undone INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP
	)`,
}

// legacyData 迁移前写入的样本数据, 迁移后必须一条不少地读回来
var legacyData = []struct {
	query string
	args  []any
}{
	{`INSERT INTO keywords (keyword, is_link, is_auto_added, added_at, source, hit_count) VALUES (?,?,?,?,?,?)`,
		[]any{"水果机", false, false, time.Now(), "manual", 7}},
	{`INSERT INTO keywords (keyword, is_link, is_auto_added, added_at, source, hit_count) VALUES (?,?,?,?,?,?)`,
		[]any{"日入1w", false, false, time.Now(), "ai", 3}},
	{`INSERT INTO keywords (keyword, is_link, is_auto_added, added_at, source, hit_count) VALUES (?,?,?,?,?,?)`,
		[]any{"legacy.example.com", true, true, time.Now().AddDate(0, -6, 0), "manual", 0}},
	{`INSERT INTO prompt_replies (prompt, reply) VALUES (?,?)`, []any{"你好", "你也好"}},
	{`INSERT INTO config (key, value) VALUES (?,?)`, []any{"binance_last_msg_id", "12345"}},
	{`INSERT INTO keyword_rejects (keyword, rejected_at) VALUES (?,?)`, []any{"多少", time.Now()}},
	{`INSERT INTO user_strikes (user_id, chat_id, strikes, last_hit_at) VALUES (?,?,?,?)`,
		[]any{int64(1001), int64(-100200), 2, time.Now()}},
	{`INSERT INTO user_stats (user_id, chat_id, message_count, first_seen_at, last_seen_at) VALUES (?,?,?,?,?)`,
		[]any{int64(1001), int64(-100200), 5, time.Now(), time.Now()}},
	{`INSERT INTO moderation_actions (user_id, chat_id, user_name, message_text, rule, learned_words, banned, undone, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		[]any{int64(1001), int64(-100200), "spammer", "水·果·特·價", "关键词", "水果机\n特价", 0, 0, time.Now()}},
}

// buildLegacyDB 造一个迁移前形态的库并塞入样本数据
func buildLegacyDB(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("打开老库失败: %v", err)
	}
	defer raw.Close()

	for _, stmt := range legacySchema {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("建老表失败 (%s): %v", stmt, err)
		}
	}
	for _, row := range legacyData {
		if _, err := raw.Exec(row.query, row.args...); err != nil {
			t.Fatalf("写入样本数据失败 (%s): %v", row.query, err)
		}
	}
	return path
}

// TestAutoMigratePreservesLegacyData 存量库跑完 AutoMigrate 后, 所有数据仍可正常读取
func TestAutoMigratePreservesLegacyData(t *testing.T) {
	path := buildLegacyDB(t)

	db, err := NewDatabaseAt(path)
	if err != nil {
		t.Fatalf("在老库上迁移失败: %v", err)
	}
	defer db.Close()

	t.Run("关键词", func(t *testing.T) {
		keywords, err := db.GetActiveKeywords()
		if err != nil {
			t.Fatalf("读取关键词失败: %v", err)
		}
		// legacy.example.com 是 is_auto_added 的遗留行, 不参与匹配
		if len(keywords) != 2 {
			t.Errorf("参与匹配的关键词数 = %d, 期望 2 (实际: %v)", len(keywords), keywords)
		}

		manual, err := db.GetKeywordsBySource(SourceManual)
		if err != nil {
			t.Fatalf("按来源读取失败: %v", err)
		}
		if len(manual) != 1 || manual[0].Word != "水果机" || manual[0].HitCount != 7 {
			t.Errorf("手工词读取错误: %+v", manual)
		}

		ai, err := db.GetKeywordsBySource(SourceAI)
		if err != nil {
			t.Fatalf("按来源读取 AI 词失败: %v", err)
		}
		if len(ai) != 1 || ai[0].Word != "日入1w" {
			t.Errorf("AI 词读取错误: %+v", ai)
		}
	})

	t.Run("提示词", func(t *testing.T) {
		prompts, err := db.GetAllPromptReplies()
		if err != nil || prompts["你好"] != "你也好" {
			t.Errorf("提示词读取错误: %v, err=%v", prompts, err)
		}
	})

	t.Run("配置", func(t *testing.T) {
		value, err := db.GetConfig("binance_last_msg_id")
		if err != nil || value != "12345" {
			t.Errorf("配置读取错误: %q, err=%v", value, err)
		}
		missing, err := db.GetConfig("不存在的键")
		if err != nil || missing != "" {
			t.Errorf("缺失配置应返回空字符串, 得到 %q, err=%v", missing, err)
		}
	})

	t.Run("计分与统计", func(t *testing.T) {
		strikes, err := db.GetStrikes(1001, -100200)
		if err != nil || strikes != 2 {
			t.Errorf("违规计分读取错误: %d, err=%v", strikes, err)
		}

		count, err := db.BumpUserMessageCount(1001, -100200)
		if err != nil || count != 6 {
			t.Errorf("发言计数累加错误: %d, err=%v", count, err)
		}
	})

	t.Run("处置记录", func(t *testing.T) {
		action, err := db.GetModerationAction(1)
		if err != nil {
			t.Fatalf("读取处置记录失败: %v", err)
		}
		if action.UserName != "spammer" || len(action.LearnedWords) != 2 {
			t.Errorf("处置记录读取错误: %+v", action)
		}
	})

	t.Run("否决表", func(t *testing.T) {
		rejected, err := db.IsKeywordRejected("多少")
		if err != nil || !rejected {
			t.Errorf("否决表读取错误: %v, err=%v", rejected, err)
		}
	})
}

// TestAutoMigrateAddsNoDuplicateColumns 迁移不应该往存量表里加重复列。
// 列名一旦和模型对不上, AutoMigrate 会静默新增一列, 老数据留在旧列里再也读不到。
func TestAutoMigrateAddsNoDuplicateColumns(t *testing.T) {
	path := buildLegacyDB(t)

	db, err := NewDatabaseAt(path)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer db.Close()

	expected := map[string][]string{
		"keywords":           {"id", "keyword", "is_link", "is_auto_added", "added_at", "source", "hit_count"},
		"prompt_replies":     {"prompt", "reply"},
		"config":             {"key", "value"},
		"keyword_rejects":    {"keyword", "rejected_at"},
		"user_strikes":       {"user_id", "chat_id", "strikes", "last_hit_at"},
		"user_stats":         {"user_id", "chat_id", "message_count", "first_seen_at", "last_seen_at"},
		"moderation_actions": {"id", "user_id", "chat_id", "user_name", "message_text", "rule", "learned_words", "banned", "undone", "created_at"},
	}

	for table, wantColumns := range expected {
		types, err := db.db.Migrator().ColumnTypes(tableModel(t, table))
		if err != nil {
			t.Errorf("读取表 %s 结构失败: %v", table, err)
			continue
		}

		got := make(map[string]bool, len(types))
		for _, c := range types {
			got[c.Name()] = true
		}

		if len(got) != len(wantColumns) {
			names := make([]string, 0, len(got))
			for name := range got {
				names = append(names, name)
			}
			t.Errorf("表 %s 列数 = %d, 期望 %d (实际: %v)", table, len(got), len(wantColumns), names)
		}
		for _, want := range wantColumns {
			if !got[want] {
				t.Errorf("表 %s 缺少列 %s", table, want)
			}
		}
	}
}

// tableModel 把表名映射回模型实例
func tableModel(t *testing.T, table string) any {
	t.Helper()
	for _, model := range allModels() {
		if namer, ok := model.(interface{ TableName() string }); ok && namer.TableName() == table {
			return model
		}
	}
	t.Fatalf("找不到表 %s 对应的模型", table)
	return nil
}

// TestFreshDatabaseWorks 全新库 (没有老 schema) 也要能建起来并正常读写
func TestFreshDatabaseWorks(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatalf("创建新库失败: %v", err)
	}
	defer db.Close()

	added, err := db.AddKeyword("水果机", SourceManual)
	if err != nil || !added {
		t.Fatalf("新增关键词失败: added=%v err=%v", added, err)
	}

	// 唯一约束必须生效, 否则同一个词会被重复插入
	again, err := db.AddKeyword("水果机", SourceManual)
	if err != nil {
		t.Fatalf("重复新增报错: %v", err)
	}
	if again {
		t.Error("重复新增应当被忽略, 但报告为新增成功")
	}

	keywords, err := db.GetActiveKeywords()
	if err != nil || len(keywords) != 1 {
		t.Errorf("关键词数 = %v (err=%v), 期望 1 条", keywords, err)
	}
}

// TestRejectBlocksAIKeyword 被否决的词, AI 不得再添加; 管理员仍可手动添加
func TestRejectBlocksAIKeyword(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "reject.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	if err := db.RejectKeyword("多少"); err != nil {
		t.Fatalf("写入否决表失败: %v", err)
	}

	added, err := db.AddKeyword("多少", SourceAI)
	if err != nil {
		t.Fatalf("AI 添加报错: %v", err)
	}
	if added {
		t.Error("被否决的词不应该被 AI 添加进去")
	}

	// 否决只约束 AI, 管理员手工添加不受限
	added, err = db.AddKeyword("多少", SourceManual)
	if err != nil || !added {
		t.Errorf("管理员手工添加被否决词应当成功: added=%v err=%v", added, err)
	}
}

// TestStrikeLifecycle 计分的累加、扣回与不减到负数
func TestStrikeLifecycle(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "strike.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	const userID, chatID = int64(42), int64(-100)

	for i := 1; i <= 3; i++ {
		got, err := db.AddStrike(userID, chatID)
		if err != nil || got != i {
			t.Fatalf("第 %d 次记分 = %d (err=%v), 期望 %d", i, got, err, i)
		}
	}

	if err := db.DecrementStrike(userID, chatID); err != nil {
		t.Fatalf("扣回计分失败: %v", err)
	}
	if got, _ := db.GetStrikes(userID, chatID); got != 2 {
		t.Errorf("扣回后计分 = %d, 期望 2", got)
	}

	for i := 0; i < 5; i++ {
		if err := db.DecrementStrike(userID, chatID); err != nil {
			t.Fatalf("扣回计分失败: %v", err)
		}
	}
	if got, _ := db.GetStrikes(userID, chatID); got != 0 {
		t.Errorf("反复扣回后计分 = %d, 期望 0 (不应为负)", got)
	}
}

// TestModerationActionUndoIsIdempotent 撤销标记必须幂等, 连点两下不能回滚两次
func TestModerationActionUndoIsIdempotent(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "action.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	id, err := db.RecordModerationAction(ModerationAction{
		UserID: 1, ChatID: -1, UserName: "spammer",
		MessageText: "水·果·特·價", Rule: "AI 判定",
		LearnedWords: []string{"水果机", "特价"},
	})
	if err != nil {
		t.Fatalf("记录处置失败: %v", err)
	}

	action, err := db.GetModerationAction(id)
	if err != nil {
		t.Fatalf("读取处置失败: %v", err)
	}
	if len(action.LearnedWords) != 2 || action.LearnedWords[0] != "水果机" {
		t.Errorf("学到的词读取错误: %v", action.LearnedWords)
	}

	claimed, err := db.MarkActionUndone(id)
	if err != nil || !claimed {
		t.Fatalf("首次撤销应当成功: claimed=%v err=%v", claimed, err)
	}
	claimed, err = db.MarkActionUndone(id)
	if err != nil {
		t.Fatalf("重复撤销报错: %v", err)
	}
	if claimed {
		t.Error("重复撤销应当返回 false, 否则会反复解封扣分")
	}
}

// TestCleanupLegacyAutoLinks 只清理遗留的自动添加链接, 不碰正常关键词
func TestCleanupLegacyAutoLinks(t *testing.T) {
	path := buildLegacyDB(t)

	db, err := NewDatabaseAt(path)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer db.Close()

	removed, err := db.CleanupLegacyAutoLinks()
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if removed != 1 {
		t.Errorf("清理条数 = %d, 期望 1 (只有 legacy.example.com 符合条件)", removed)
	}

	keywords, _ := db.GetActiveKeywords()
	if len(keywords) != 2 {
		t.Errorf("清理后参与匹配的关键词数 = %d, 期望 2 (不应误删)", len(keywords))
	}
}
