import json
import os
import logging
from database import Database

# 设置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

def migrate_data(json_file, db_file):
    try:
        # 确保 data 目录存在
        os.makedirs(os.path.dirname(db_file), exist_ok=True)
        logger.info(f"Ensuring directory exists: {os.path.dirname(db_file)}")

        # 创建数据库连接
        db = Database(db_file)
        logger.info(f"Database connection created: {db_file}")

        # 读取 JSON 文件
        with open(json_file, 'r') as f:
            keywords = json.load(f)
        logger.info(f"JSON file loaded: {json_file}")

        if not isinstance(keywords, list):
            raise ValueError(f"Expected a list in JSON file, but got {type(keywords)}")

        # 迁移关键词
        for keyword in keywords:
            db.add_keyword(keyword)
        
        logger.info(f"Migration complete. Migrated {len(keywords)} keywords.")

        # 验证迁移
        migrated_keywords = db.get_all_keywords()
        logger.info(f"Verified {len(migrated_keywords)} keywords in the database.")

    except Exception as e:
        logger.error(f"An error occurred during migration: {str(e)}")
        raise

if __name__ == "__main__":
    json_file = '/app/data/keywords.json'  # 旧的 JSON 文件路径
    db_file = '/app/data/q58.db'  # 新的数据库文件路径
    migrate_data(json_file, db_file)
