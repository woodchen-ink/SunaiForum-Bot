import os
import logging
import asyncio
import time
from telethon import TelegramClient, events
from collections import deque
from link_filter import LinkFilter
from bot_commands import handle_keyword_command, handle_whitelist_command, get_keywords

# 环境变量
BOT_TOKEN = os.environ.get('BOT_TOKEN')
ADMIN_ID = int(os.environ.get('ADMIN_ID'))
KEYWORDS_FILE = '/app/data/keywords.json'
WHITELIST_FILE = '/app/data/whitelist.json'

# 设置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger('TeleGuard')

# 创建 LinkFilter 实例
link_filter = LinkFilter('/app/data/keywords.json', '/app/data/whitelist.json')

# 限速器
class RateLimiter:
    def __init__(self, max_calls, period):
        """
        初始化RateLimiter类的实例。

        参数:
        max_calls (int): 限制的最大调用次数。
        period (float): 限定的时间周期（秒）。

        该构造函数设置了速率限制器的基本参数，并初始化了一个双端队列，
        用于跟踪调用的时间点，以 enforcement of the rate limiting policy。
        """
        # 限制的最大调用次数
        self.max_calls = max_calls
        # 限定的时间周期（秒）
        self.period = period
        # 用于存储调用时间的双端队列
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

# 延迟删除消息函数
async def delete_message_after_delay(client, chat, message, delay):
    # 延迟指定的时间
    await asyncio.sleep(delay)
    
    # 尝试删除消息
    try:
        await client.delete_messages(chat, message)
    except Exception as e:
        # 如果删除失败，记录错误
        logger.error(f"Failed to delete message: {e}")

# 处理消息函数
async def process_message(event, client):
    if not event.is_private:
        # 检查消息是否包含需要过滤的链接
        if link_filter.should_filter(event.message.text):
            if event.sender_id != ADMIN_ID:
                await event.delete()
                notification = await event.respond("已撤回该消息。注:重复发送的链接会被自动撤回。")
                asyncio.create_task(delete_message_after_delay(client, event.chat_id, notification, 30 * 60))
            return

        # 检查关键词
        keywords = get_keywords()
        if any(keyword in event.message.text.lower() for keyword in keywords):
            if event.sender_id != ADMIN_ID:
                await event.delete()
                notification = await event.respond("已撤回该消息。注:已发送的推广链接不要多次发送,置顶已有项目的推广链接也会自动撤回。")
                asyncio.create_task(delete_message_after_delay(client, event.chat_id, notification, 30 * 60))

# 启动机器人函数
async def start_bot():
    client = TelegramClient('bot', api_id=6, api_hash='eb06d4abfb49dc3eeb1aeb98ae0f581e')
    await client.start(bot_token=BOT_TOKEN)
    
    @client.on(events.NewMessage(pattern='/add|/delete|/list'))
    async def keyword_handler(event):
        await handle_keyword_command(event, client)
        link_filter.reload_keywords()  # 重新加载关键词

    @client.on(events.NewMessage(pattern='/addwhite|/delwhite|/listwhite'))
    async def whitelist_handler(event):
        await handle_whitelist_command(event, client)
        link_filter.reload_whitelist()  # 重新加载白名单

    @client.on(events.NewMessage(pattern=''))
    async def message_handler(event):
        if not event.is_private or event.sender_id != ADMIN_ID:
            async with rate_limiter:
                await process_message(event, client)

    logger.info("TeleGuard is running...")
    await client.run_until_disconnected()
    
# 主函数
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
