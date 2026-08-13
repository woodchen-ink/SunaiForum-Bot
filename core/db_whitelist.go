package core

// whitelist 表读写; 存放允许在群内出现的域名

// AddWhitelist 把域名加入白名单, 已存在则静默忽略
func (d *Database) AddWhitelist(domain string) error {
	_, err := d.db.Exec("INSERT OR IGNORE INTO whitelist (domain) VALUES (?)", domain)
	if err != nil {
		return err
	}
	d.invalidateCache(cacheWhitelist)
	return nil
}

func (d *Database) RemoveWhitelist(domain string) error {
	_, err := d.db.Exec("DELETE FROM whitelist WHERE domain = ?", domain)
	if err != nil {
		return err
	}
	d.invalidateCache(cacheWhitelist)
	return nil
}

// GetAllWhitelist 返回全部白名单域名, 走 TTL 缓存; 增删域名会立即失效缓存
func (d *Database) GetAllWhitelist() ([]string, error) {
	return d.queryCached(cacheWhitelist, "SELECT domain FROM whitelist")
}

func (d *Database) WhitelistExists(domain string) (bool, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM whitelist WHERE domain = ?", domain).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
