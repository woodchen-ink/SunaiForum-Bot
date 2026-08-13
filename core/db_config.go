package core

// config 表读写; 存放需要跨容器重启保留的运行时状态 (键值对)
import "fmt"

func (d *Database) SetConfig(key, value string) error {
	_, err := d.db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", key, value)
	if err != nil {
		return fmt.Errorf("写入配置 %s 失败: %w", key, err)
	}
	return nil
}

// GetConfig 读取配置项; 键不存在时返回空字符串而非错误
func (d *Database) GetConfig(key string) (string, error) {
	var value string
	err := d.db.QueryRow("SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err != nil {
		if isNoRows(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	return value, nil
}

func (d *Database) DeleteConfig(key string) error {
	_, err := d.db.Exec("DELETE FROM config WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("删除配置 %s 失败: %w", key, err)
	}
	return nil
}
