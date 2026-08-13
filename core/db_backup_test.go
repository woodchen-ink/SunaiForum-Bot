package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBackupProducesReadableSnapshot 快照必须是一个能独立打开、数据完整的库
func TestBackupProducesReadableSnapshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sunai.db")
	db, err := NewDatabaseAt(dbPath)
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	for _, word := range []string{"水果机", "日入1w", "代收款"} {
		if _, err := db.AddKeyword(word, SourceManual); err != nil {
			t.Fatalf("写入关键词失败: %v", err)
		}
	}

	snapshot, err := db.Backup("test", 7)
	if err != nil {
		t.Fatalf("生成快照失败: %v", err)
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatalf("快照文件不存在: %v", err)
	}

	// 快照要能作为一个独立的库打开, 且数据一条不差
	restored, err := NewDatabaseAt(snapshot)
	if err != nil {
		t.Fatalf("打开快照失败: %v", err)
	}
	defer restored.Close()

	keywords, err := restored.GetActiveKeywords()
	if err != nil {
		t.Fatalf("从快照读取关键词失败: %v", err)
	}
	if len(keywords) != 3 {
		t.Errorf("快照里的关键词数 = %d, 期望 3 (实际: %v)", len(keywords), keywords)
	}
}

// TestBackupRotation 只保留最近 N 份, 且不碰不是自己产生的文件
func TestBackupRotation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sunai.db")
	db, err := NewDatabaseAt(dbPath)
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	backupDir := filepath.Join(filepath.Dir(dbPath), backupDirName)
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		t.Fatalf("创建快照目录失败: %v", err)
	}

	// 管理员手工放进来的文件不该被轮转吃掉
	manual := filepath.Join(backupDir, "手工备份-勿删.db")
	if err := os.WriteFile(manual, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("写入手工备份失败: %v", err)
	}

	// 造 5 份历史快照 (文件名带时间戳, 字典序即时间序)
	for _, stamp := range []string{"20260101-000000", "20260102-000000", "20260103-000000", "20260104-000000", "20260105-000000"} {
		name := backupPrefix + stamp + "-daily" + backupSuffix
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte("old"), 0o644); err != nil {
			t.Fatalf("写入历史快照失败: %v", err)
		}
	}

	if err := rotateBackups(backupDir, 3); err != nil {
		t.Fatalf("轮转失败: %v", err)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("读取快照目录失败: %v", err)
	}

	var snapshots []string
	manualSurvived := false
	for _, entry := range entries {
		switch {
		case strings.HasPrefix(entry.Name(), backupPrefix):
			snapshots = append(snapshots, entry.Name())
		case entry.Name() == "手工备份-勿删.db":
			manualSurvived = true
		}
	}

	if len(snapshots) != 3 {
		t.Errorf("轮转后快照数 = %d, 期望 3 (实际: %v)", len(snapshots), snapshots)
	}
	// 保留的应当是最新的三份
	for _, want := range []string{"20260103", "20260104", "20260105"} {
		found := false
		for _, name := range snapshots {
			if strings.Contains(name, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("应当保留的快照 %s 被删掉了", want)
		}
	}
	if !manualSurvived {
		t.Error("轮转不该删除管理员手工放入的备份文件")
	}
}

// TestRotationKeepZeroDisablesCleanup keep <= 0 表示不清理
func TestRotationKeepZeroDisablesCleanup(t *testing.T) {
	backupDir := t.TempDir()
	for _, stamp := range []string{"20260101-000000", "20260102-000000"} {
		name := backupPrefix + stamp + "-daily" + backupSuffix
		if err := os.WriteFile(filepath.Join(backupDir, name), []byte("x"), 0o644); err != nil {
			t.Fatalf("写入快照失败: %v", err)
		}
	}

	if err := rotateBackups(backupDir, 0); err != nil {
		t.Fatalf("轮转报错: %v", err)
	}

	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 2 {
		t.Errorf("keep=0 时不该删除任何文件, 剩余 %d 个", len(entries))
	}
}

// TestBackupBeforeMigrateSkipsFreshDatabase 新库没有备份价值, 不该产生空快照
func TestBackupBeforeMigrateSkipsFreshDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sunai.db")

	db, err := NewDatabaseAt(dbPath)
	if err != nil {
		t.Fatalf("创建库失败: %v", err)
	}
	defer db.Close()

	// 手动模拟"库文件不存在"的启动场景
	missing := &Database{db: db.db, path: filepath.Join(dir, "not-exist.db")}
	missing.backupBeforeMigrate(7)

	if _, err := os.Stat(filepath.Join(dir, backupDirName)); err == nil {
		t.Error("库文件不存在时不该创建快照目录")
	}
}
