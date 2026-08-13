package core

// 迁移完整性护栏。
//
// 背景: 2026-08-13 的 GORM 迁移中, AutoMigrate 对 keywords 表做了整表重建,
// 拷贝时漏掉了 keyword 与 added_at 两列, 4 条关键词静默丢失。事后无法在本地复现,
// 也就无法用测试排除它再次发生。
//
// 结论是不再指望"证明迁移安全", 而是保证"迁移出事时立刻可见、且能回滚":
//   1. 迁移前打快照 (db_backup.go)
//   2. 迁移前后比对关键指标, 缩水就大声报警并指出快照路径
//
// 护栏只报警不阻断: 迁移已经发生, 阻断启动救不回数据, 反而让机器人也停摆。
import (
	"fmt"
	"log"
)

// tableProbe 一张表在迁移前后都要比对的指标
type tableProbe struct {
	table string
	// column 该表的核心内容列; 行还在但这一列变空, 正是本次事故的形态
	column string
}

// guardedProbes 需要护栏保护的表。只列内容有价值、丢了无法自动重建的。
var guardedProbes = []tableProbe{
	{table: "keywords", column: "keyword"},
	{table: "prompt_replies", column: "reply"},
	{table: "keyword_rejects", column: "keyword"},
}

// probeResult 一次采样结果
type probeResult struct {
	rows    int64
	nonNull int64
}

// probeTables 采集各表的行数与核心列非空数; 表不存在时跳过 (老库里本就没有新表)
func (d *Database) probeTables() map[string]probeResult {
	results := make(map[string]probeResult, len(guardedProbes))

	for _, probe := range guardedProbes {
		var result probeResult
		row := d.db.Raw(fmt.Sprintf(
			"SELECT COUNT(*), COUNT(%s) FROM %s", probe.column, probe.table)).Row()
		if err := row.Scan(&result.rows, &result.nonNull); err != nil {
			continue // 表还不存在, 或本轮迁移才会创建
		}
		results[probe.table] = result
	}
	return results
}

// verifyMigration 比对迁移前后的采样, 发现数据缩水就大声报警。
// snapshotPath 为迁移前快照的位置, 为空表示这次没有快照 (新库或快照失败)。
func (d *Database) verifyMigration(before map[string]probeResult, snapshotPath string) {
	after := d.probeTables()

	for table, pre := range before {
		post, ok := after[table]
		if !ok {
			logDataLoss(table, "表在迁移后消失", snapshotPath)
			continue
		}

		if post.rows < pre.rows {
			logDataLoss(table, fmt.Sprintf("行数 %d -> %d", pre.rows, post.rows), snapshotPath)
		}
		// 行还在但内容列变空, 正是 2026-08-13 事故的形态, 单看行数发现不了
		if post.nonNull < pre.nonNull {
			logDataLoss(table, fmt.Sprintf("有效内容 %d -> %d 条", pre.nonNull, post.nonNull), snapshotPath)
		}
	}
}

// logDataLoss 用醒目格式记录疑似数据丢失, 便于在日志里一眼看到
func logDataLoss(table, detail, snapshotPath string) {
	log.Printf("[Database] !!!!! 迁移疑似丢失数据 !!!!!  表 %s: %s", table, detail)
	if snapshotPath != "" {
		log.Printf("[Database] !!!!! 迁移前快照在 %s, 停机后用它覆盖 sunai.db 即可回滚", snapshotPath)
	} else {
		log.Printf("[Database] !!!!! 本次没有迁移前快照, 无法回滚")
	}
}
