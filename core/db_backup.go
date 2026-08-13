package core

// 数据库快照。
//
// 用 SQLite 的 VACUUM INTO 而不是在宿主机上 cp: VACUUM INTO 走当前连接、由 SQLite 保证
// 一致性快照, 而直接复制正在被写入的库文件可能拿到撕裂状态。这也是宿主机层面的定时 cp
// 和 Dokploy volume backup (它只认 docker volume, 不认 bind mount) 都不合适的原因。
//
// 两个触发点:
//   - 启动时、AutoMigrate 之前 —— schema 变更是最危险的时刻, 出问题能直接拿快照回滚
//   - 每天一次 —— 词表随 AI 自动扩充而积累, 值得留历史
import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// backupDirName 快照存放目录, 位于数据卷内, 随卷一起持久化
	backupDirName = "backups"
	// backupPrefix / backupSuffix 快照文件名格式, 用于识别和轮转自己产生的文件
	backupPrefix = "sunai-"
	backupSuffix = ".db"
)

// Backup 生成一份一致性快照并轮转旧文件, 返回快照路径。
// label 会写进文件名, 用于区分启动前快照与每日快照。
func (d *Database) Backup(label string, keep int) (string, error) {
	backupDir := filepath.Join(filepath.Dir(d.path), backupDirName)
	if err := os.MkdirAll(backupDir, os.ModePerm); err != nil {
		return "", fmt.Errorf("创建快照目录失败: %w", err)
	}

	name := fmt.Sprintf("%s%s-%s%s", backupPrefix, time.Now().Format("20060102-150405"), label, backupSuffix)
	target := filepath.Join(backupDir, name)

	// VACUUM INTO 要求目标文件不存在, 时间戳命名天然满足
	if err := d.db.Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", fmt.Errorf("生成快照失败: %w", err)
	}

	if err := rotateBackups(backupDir, keep); err != nil {
		// 轮转失败不影响本次快照已经成功这个事实
		log.Printf("[Database] 轮转旧快照失败: %v", err)
	}

	return target, nil
}

// rotateBackups 只保留最近 keep 份快照; keep <= 0 表示不清理
func rotateBackups(dir string, keep int) error {
	if keep <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var names []string
	for _, entry := range entries {
		name := entry.Name()
		// 只动自己产生的文件, 不碰管理员手工放进来的备份
		if !entry.IsDir() && strings.HasPrefix(name, backupPrefix) && strings.HasSuffix(name, backupSuffix) {
			names = append(names, name)
		}
	}
	if len(names) <= keep {
		return nil
	}

	// 文件名带时间戳, 字典序即时间序
	sort.Strings(names)
	for _, name := range names[:len(names)-keep] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
		log.Printf("[Database] 已清理旧快照 %s", name)
	}
	return nil
}

// backupBeforeMigrate 在 schema 变更前留一份快照, 返回快照路径 (没打成则为空)。
// 新库 (文件不存在或为空) 没有备份价值, 直接跳过; 失败只告警不阻断启动 ——
// 备份不该成为机器人起不来的理由, 但要在日志里说清这次没有安全网。
func (d *Database) backupBeforeMigrate(keep int) string {
	info, err := os.Stat(d.path)
	if err != nil || info.Size() == 0 {
		return ""
	}

	path, err := d.Backup("premigrate", keep)
	if err != nil {
		log.Printf("[Database] 迁移前快照失败, 本次迁移没有回滚点: %v", err)
		return ""
	}
	log.Printf("[Database] 迁移前快照已生成: %s", filepath.Base(path))
	return path
}
