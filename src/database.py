import sqlite3
import logging
import os
import time

logger = logging.getLogger(__name__)

class Database:
    def __init__(self, db_file):
        self.db_file = db_file
        os.makedirs(os.path.dirname(db_file), exist_ok=True)
        self._keywords_cache = None
        self._whitelist_cache = None
        self._cache_time = 0
        self.create_tables()

    def create_tables(self):
        with sqlite3.connect(self.db_file) as conn:
            cursor = conn.cursor()
            # 创建关键词表并添加索引
            cursor.execute('''
                CREATE TABLE IF NOT EXISTS keywords
                (id INTEGER PRIMARY KEY, keyword TEXT UNIQUE)
            ''')
            cursor.execute('CREATE INDEX IF NOT EXISTS idx_keyword ON keywords(keyword)')
            
            # 创建白名单表并添加索引
            cursor.execute('''
                CREATE TABLE IF NOT EXISTS whitelist
                (id INTEGER PRIMARY KEY, domain TEXT UNIQUE)
            ''')
            cursor.execute('CREATE INDEX IF NOT EXISTS idx_domain ON whitelist(domain)')
            
            # 创建全文搜索虚拟表
            cursor.execute('''
                CREATE VIRTUAL TABLE IF NOT EXISTS keywords_fts USING fts5(keyword)
            ''')
            conn.commit()

    def execute_query(self, query, params=None):
        with sqlite3.connect(self.db_file) as conn:
            cursor = conn.cursor()
            if params:
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            conn.commit()
            return cursor.fetchall()

    def add_keyword(self, keyword):
        self.execute_query("INSERT OR IGNORE INTO keywords (keyword) VALUES (?)", (keyword,))
        self.execute_query("INSERT OR IGNORE INTO keywords_fts (keyword) VALUES (?)", (keyword,))
        self._invalidate_cache()

    def remove_keyword(self, keyword):
        self.execute_query("DELETE FROM keywords WHERE keyword = ?", (keyword,))
        self.execute_query("DELETE FROM keywords_fts WHERE keyword = ?", (keyword,))
        self._invalidate_cache()

    def get_all_keywords(self):
        current_time = time.time()
        if self._keywords_cache is None or current_time - self._cache_time > 300:  # 5分钟缓存
            self._keywords_cache = [row[0] for row in self.execute_query("SELECT keyword FROM keywords")]
            self._cache_time = current_time
        return self._keywords_cache

    def remove_keywords_containing(self, substring):
        query = "DELETE FROM keywords WHERE keyword LIKE ?"
        result = self.execute_query(query, (f"%{substring}%",))
        self.execute_query("DELETE FROM keywords_fts WHERE keyword LIKE ?", (f"%{substring}%",))
        self._invalidate_cache()
        return result

    def add_whitelist(self, domain):
        self.execute_query("INSERT OR IGNORE INTO whitelist (domain) VALUES (?)", (domain,))
        self._invalidate_cache()

    def remove_whitelist(self, domain):
        self.execute_query("DELETE FROM whitelist WHERE domain = ?", (domain,))
        self._invalidate_cache()

    def get_all_whitelist(self):
        current_time = time.time()
        if self._whitelist_cache is None or current_time - self._cache_time > 300:  # 5分钟缓存
            self._whitelist_cache = [row[0] for row in self.execute_query("SELECT domain FROM whitelist")]
            self._cache_time = current_time
        return self._whitelist_cache

    def search_keywords(self, pattern):
        return [row[0] for row in self.execute_query("SELECT keyword FROM keywords_fts WHERE keyword MATCH ?", (pattern,))]

    def _invalidate_cache(self):
        self._keywords_cache = None
        self._whitelist_cache = None
        self._cache_time = 0
