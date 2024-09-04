import os
import logging
import asyncio
from functools import partial
from telethon import TelegramClient, events
from collections import deque
import time
from link_filter import LinkFilter
from bot_commands import handle_command
import logging

# 设置日志
DEBUG_MODE = os.environ.get('DEBUG_MODE', 'False').lower() == 'true'

logging.basicConfig(level=logging.INFO if not DEBUG_MODE else logging.DEBUG, 
                    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

logger = logging.getLogger('TeleGuard')
link_filter_logger = logging.getLogger('TeleGuard.LinkFilter')
link_filter_logger.setLevel(logging.DEBUG if DEBUG_MODE else logging.INFO)

# 调整第三方库的日志级别
logging.getLogger('telethon').setLevel(logging.WARNING)

# 环境变量
BOT_TOKEN = os.environ.get('BOT_TOKEN')
ADMIN_ID = int(os.environ.get('ADMIN_ID'))
KEYWORDS_FILE = '/app/data/keywords.json'
WHITELIST_FILE = '/app/data/whitelist.json'

# 设置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger('TeleGuard')

# 创建 LinkFilter 实例
link_filter = LinkFilter(KEYWORDS_FILE, WHITELIST_FILE)

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

rate_limiter = RateLimiter(max_calls=10, period=1)

async def delete_message_after_delay(client, chat, message, delay):
    await asyncio.sleep(delay)
    try:
        await client.delete_messages(chat, message)
    except Exception as e:
        logger.error(f"Failed to delete message: {e}")

async def process_message(event, client):
    if not event.is_private:
        logger.debug(f"Processing message: {event.message.text}")
        should_filter, new_links = link_filter.should_filter(event.message.text)
        if should_filter:
            logger.info(f"Message should be filtered: {event.message.text}")
            if event.sender_id != ADMIN_ID:
                await event.delete()
                notification = await event.respond("已撤回该消息。注:包含关键词或重复发送的非白名单链接会被自动撤回。")
                asyncio.create_task(delete_message_after_delay(client, event.chat_id, notification, 3 * 60))
            return
        if new_links:
            logger.info(f"New non-whitelisted links found: {new_links}")


async def command_handler(event, link_filter):
    if event.is_private and event.sender_id == ADMIN_ID:
        await handle_command(event, event.client)
        if event.raw_text.startswith(('/add', '/delete', '/list', '/addwhite', '/delwhite', '/listwhite')):
            link_filter.load_data_from_file()

async def message_handler(event, link_filter, rate_limiter):
    if not event.is_private or event.sender_id != ADMIN_ID:
        async with rate_limiter:
            await process_message(event, event.client)

async def start_bot():
    async with TelegramClient('bot', api_id=6, api_hash='eb06d4abfb49dc3eeb1aeb98ae0f581e') as client:
        await client.start(bot_token=BOT_TOKEN)
        
        client.add_event_handler(
            partial(command_handler, link_filter=link_filter),
            events.NewMessage(pattern='/add|/delete|/list|/addwhite|/delwhite|/listwhite')
        )
        client.add_event_handler(
            partial(message_handler, link_filter=link_filter, rate_limiter=rate_limiter),
            events.NewMessage()
        )

        logger.info("TeleGuard is running...")
        await client.run_until_disconnected()

async def main():
    while True:
        try:
            await start_bot()
        except (KeyboardInterrupt, SystemExit):
            logger.info("TeleGuard is shutting down...")
            break
        except Exception as e:
            logger.error(f"An error occurred in TeleGuard: {str(e)}")
            logger.info("Attempting to restart TeleGuard in 60 seconds...")
            await asyncio.sleep(60)

def run():
    asyncio.run(main())

if __name__ == '__main__':
    run()
