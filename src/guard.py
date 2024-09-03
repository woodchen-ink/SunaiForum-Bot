import os
import json
import logging
import asyncio
import time
from telethon import TelegramClient, events
from collections import deque

# 环境变量
BOT_TOKEN = os.environ.get('BOT_TOKEN')
ADMIN_ID = int(os.environ.get('ADMIN_ID'))
KEYWORDS_FILE = '/app/data/keywords.json'

# 设置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger('TeleGuard')

def load_keywords():
    if os.path.exists(KEYWORDS_FILE):
        with open(KEYWORDS_FILE, 'r') as f:
            return json.load(f)
    return ['推广', '广告', 'ad', 'promotion']

def save_keywords(keywords):
    with open(KEYWORDS_FILE, 'w') as f:
        json.dump(keywords, f)

KEYWORDS = load_keywords()

class RateLimiter:
    def __init__(self, max_calls, period):
        self.max_calls = max_calls
        self.period = period
        self.calls = deque()

    async def __aenter__(self):
        now = time.time()
        while self.calls and now - self.calls[0] >= self.period:
            self.calls.popleft()
        if len(self.calls) >= self.max_calls:
            await asyncio.sleep(self.period - (now - self.calls[0]))
        self.calls.append(time.time())
        return self

    async def __aexit__(self, *args):
        pass


rate_limiter = RateLimiter(max_calls=10, period=1)  # 每秒最多处理10条消息

async def process_message(event):
    if not event.is_private and any(keyword in event.message.text.lower() for keyword in KEYWORDS):
        if event.sender_id != ADMIN_ID:
            await event.delete()
            await event.respond("已撤回该消息。注:已发送的推广链接不要多次发送,置顶已有项目的推广链接也会自动撤回。")

async def start_bot():
    client = TelegramClient('bot', api_id=6, api_hash='eb06d4abfb49dc3eeb1aeb98ae0f581e')
    await client.start(bot_token=BOT_TOKEN)
    
    @client.on(events.NewMessage(pattern=''))
    async def handler(event):
        global KEYWORDS
        if event.is_private and event.sender_id == ADMIN_ID:
            command = event.message.text.split(maxsplit=1)
            if command[0].lower() == '/add' and len(command) > 1:
                new_keyword = command[1].lower()
                if new_keyword not in KEYWORDS:
                    KEYWORDS.append(new_keyword)
                    save_keywords(KEYWORDS)
                    await event.respond(f"关键词或语句 '{new_keyword}' 已添加到列表中。")
                else:
                    await event.respond(f"关键词或语句 '{new_keyword}' 已经在列表中。")
            elif command[0].lower() == '/delete' and len(command) > 1:
                keyword_to_delete = command[1].lower()
                if keyword_to_delete in KEYWORDS:
                    KEYWORDS.remove(keyword_to_delete)
                    save_keywords(KEYWORDS)
                    await event.respond(f"关键词或语句 '{keyword_to_delete}' 已从列表中删除。")
                else:
                    await event.respond(f"关键词或语句 '{keyword_to_delete}' 不在列表中。")
            elif command[0].lower() == '/list':
                await event.respond(f"当前关键词和语句列表：\n" + "\n".join(KEYWORDS))
            return

        async with rate_limiter:
            await process_message(event)

    logger.info("TeleGuard is running...")
    await client.run_until_disconnected()

def run():
    while True:
        try:
            asyncio.get_event_loop().run_until_complete(start_bot())
        except Exception as e:
            logger.error(f"An error occurred in TeleGuard: {str(e)}")
            logger.info("Attempting to restart TeleGuard in 60 seconds...")
            time.sleep(60)  # 等待60秒后重试

if __name__ == '__main__':
    run()
