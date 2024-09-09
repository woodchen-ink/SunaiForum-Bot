import sqlite3
import logging
import os

logger = logging.getLogger(__name__)

class Database:
    def __init__(self, db_file):
        self.db_file = db_file
        os.makedirs(os.path.dirname(db_file), exist_ok=True)
        self.create_tables()

    def create_tables(self):
        with sqlite3.connect(self.db_file) as conn:
            cursor = conn.cursor()
            cursor.execute('''
                CREATE TABLE IF NOT EXISTS keywords
                (id INTEGER PRIMARY KEY, keyword TEXT UNIQUE)
            ''')
            cursor.execute('''
                CREATE TABLE IF NOT EXISTS whitelist
                (id INTEGER PRIMARY KEY, domain TEXT UNIQUE)
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
        query = "INSERT OR IGNORE INTO keywords (keyword) VALUES (?)"
        self.execute_query(query, (keyword,))

    def remove_keyword(self, keyword):
        query = "DELETE FROM keywords WHERE keyword = ?"
        self.execute_query(query, (keyword,))

    def get_all_keywords(self):
        query = "SELECT keyword FROM keywords"
        results = self.execute_query(query)
        return [row[0] for row in results]

    def remove_keywords_containing(self, substring):
        query = "DELETE FROM keywords WHERE keyword LIKE ?"
        return self.execute_query(query, (f"%{substring}%",))

    def add_whitelist(self, domain):
        query = "INSERT OR IGNORE INTO whitelist (domain) VALUES (?)"
        self.execute_query(query, (domain,))

    def remove_whitelist(self, domain):
        query = "DELETE FROM whitelist WHERE domain = ?"
        self.execute_query(query, (domain,))

    def get_all_whitelist(self):
        query = "SELECT domain FROM whitelist"
        results = self.execute_query(query)
        return [row[0] for row in results]
