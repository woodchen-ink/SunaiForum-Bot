package core

// 护栏本身必须可信: 它是"迁移丢数据"这类事故的唯一发现手段, 静默失效等于没有。
import (
	"path/filepath"
	"testing"
)

func TestProbeTablesCountsContent(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "probe.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	for _, word := range []string{"水果机", "日入1w"} {
		if _, err := db.AddKeyword(word, SourceManual); err != nil {
			t.Fatalf("写入关键词失败: %v", err)
		}
	}

	probes := db.probeTables()
	got, ok := probes["keywords"]
	if !ok {
		t.Fatal("没有采集到 keywords 表")
	}
	if got.rows != 2 || got.nonNull != 2 {
		t.Errorf("采样结果 = %+v, 期望 rows=2 nonNull=2", got)
	}
}

// TestProbeDetectsNulledColumn 这正是 2026-08-13 事故的形态:
// 行数没变, 但内容列被清空。只看行数发现不了, 必须看非空计数。
func TestProbeDetectsNulledColumn(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "nulled.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	for _, word := range []string{"水果机", "日入1w", "代收款"} {
		if _, err := db.AddKeyword(word, SourceManual); err != nil {
			t.Fatalf("写入关键词失败: %v", err)
		}
	}

	before := db.probeTables()
	if before["keywords"].nonNull != 3 {
		t.Fatalf("迁移前非空数 = %d, 期望 3", before["keywords"].nonNull)
	}

	// 模拟事故: 行还在, keyword 列被清空
	if err := db.db.Exec("UPDATE keywords SET keyword = NULL").Error; err != nil {
		t.Fatalf("模拟清空失败: %v", err)
	}

	after := db.probeTables()
	if after["keywords"].rows != 3 {
		t.Errorf("行数不该变化, 得到 %d", after["keywords"].rows)
	}
	if after["keywords"].nonNull != 0 {
		t.Errorf("非空数应当归零, 得到 %d", after["keywords"].nonNull)
	}
	if after["keywords"].nonNull >= before["keywords"].nonNull {
		t.Error("护栏未能识别出内容被清空 —— 这正是它存在的意义")
	}
}

// TestProbeSkipsMissingTables 老库里没有的新表不该让采样报错
func TestProbeSkipsMissingTables(t *testing.T) {
	path := buildLegacyDB(t)

	db, err := NewDatabaseAt(path)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer db.Close()

	// 迁移后新表已建出, 采样应当覆盖到, 且不 panic
	probes := db.probeTables()
	if _, ok := probes["keyword_rejects"]; !ok {
		t.Error("迁移后应当能采集到 keyword_rejects 表")
	}
}

// TestVerifyMigrationSurvivesMissingSnapshot 没有快照时护栏仍要能跑完, 只是提示无法回滚
func TestVerifyMigrationSurvivesMissingSnapshot(t *testing.T) {
	db, err := NewDatabaseAt(filepath.Join(t.TempDir(), "verify.db"))
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	if _, err := db.AddKeyword("水果机", SourceManual); err != nil {
		t.Fatalf("写入关键词失败: %v", err)
	}

	before := db.probeTables()
	db.db.Exec("UPDATE keywords SET keyword = NULL")

	// 只要不 panic 即可; 护栏的职责是报警, 不是阻断
	db.verifyMigration(before, "")
	db.verifyMigration(before, "/some/snapshot.db")
}
