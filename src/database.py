import sqlite3
import logging

logger = logging.getLogger(__name__)


class Database:
    def __init__(self, db_file):
        self.db_file = db_file
        self.conn = None
        self.create_tables()

    def create_tables(self):
        try:
            self.conn = sqlite3.connect(self.db_file)
            cursor = self.conn.cursor()
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS keywords
                (id INTEGER PRIMARY KEY, keyword TEXT UNIQUE)
            """
            )
            cursor.execute(
                """
                CREATE TABLE IF NOT EXISTS whitelist
                (id INTEGER PRIMARY KEY, domain TEXT UNIQUE)
            """
            )
            self.conn.commit()
        except sqlite3.Error as e:
            logger.error(f"Database error: {e}")
        finally:
            if self.conn:
                self.conn.close()

    def execute_query(self, query, params=None):
        try:
            self.conn = sqlite3.connect(self.db_file)
            cursor = self.conn.cursor()
            if params:
                cursor.execute(query, params)
            else:
                cursor.execute(query)
            self.conn.commit()
            return cursor
        except sqlite3.Error as e:
            logger.error(f"Query execution error: {e}")
            return None
        finally:
            if self.conn:
                self.conn.close()

    def add_keyword(self, keyword):
        query = "INSERT OR IGNORE INTO keywords (keyword) VALUES (?)"
        self.execute_query(query, (keyword,))

    def remove_keyword(self, keyword):
        query = "DELETE FROM keywords WHERE keyword = ?"
        self.execute_query(query, (keyword,))

    def get_all_keywords(self):
        query = "SELECT keyword FROM keywords"
        cursor = self.execute_query(query)
        return [row[0] for row in cursor] if cursor else []

    def remove_keywords_containing(self, substring):
        query = "DELETE FROM keywords WHERE keyword LIKE ?"
        cursor = self.execute_query(query, (f"%{substring}%",))
        return cursor.rowcount if cursor else 0

    def add_whitelist(self, domain):
        query = "INSERT OR IGNORE INTO whitelist (domain) VALUES (?)"
        self.execute_query(query, (domain,))

    def remove_whitelist(self, domain):
        query = "DELETE FROM whitelist WHERE domain = ?"
        self.execute_query(query, (domain,))

    def get_all_whitelist(self):
        query = "SELECT domain FROM whitelist"
        cursor = self.execute_query(query)
        return [row[0] for row in cursor] if cursor else []
