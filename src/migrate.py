import json
import os
from database import Database

def migrate_data(json_file, db_file):
    # 确保 data 目录存在
    os.makedirs(os.path.dirname(db_file), exist_ok=True)

    # 创建数据库连接
    db = Database(db_file)

    # 读取 JSON 文件
    with open(json_file, 'r') as f:
        data = json.load(f)

    # 迁移关键词
    for keyword in data.get('keywords', []):
        db.add_keyword(keyword)

    # 迁移白名单
    for domain in data.get('whitelist', []):
        db.add_whitelist(domain)

    print(f"迁移完成。关键词：{len(data.get('keywords', []))}个，白名单：{len(data.get('whitelist', []))}个")

if __name__ == "__main__":
    json_file = 'keywords.json'  # 旧的 JSON 文件路径
    db_file = os.path.join('data', 'q58.db')  # 新的数据库文件路径
    migrate_data(json_file, db_file)
