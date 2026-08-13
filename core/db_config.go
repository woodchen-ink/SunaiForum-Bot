package core

// config 表读写; 存放需要跨容器重启保留的运行时状态 (键值对)
import (
	"fmt"

	"gorm.io/gorm/clause"
)

func (d *Database) SetConfig(key, value string) error {
	err := d.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&Config{Key: key, Value: value}).Error
	if err != nil {
		return fmt.Errorf("写入配置 %s 失败: %w", key, err)
	}
	return nil
}

// GetConfig 读取配置项; 键不存在时返回空字符串而非错误
func (d *Database) GetConfig(key string) (string, error) {
	var row Config
	err := d.db.Where("key = ?", key).First(&row).Error
	if err != nil {
		if isNoRows(err) {
			return "", nil
		}
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	return row.Value, nil
}

func (d *Database) DeleteConfig(key string) error {
	if err := d.db.Where("key = ?", key).Delete(&Config{}).Error; err != nil {
		return fmt.Errorf("删除配置 %s 失败: %w", key, err)
	}
	return nil
}
