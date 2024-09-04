import os
import multiprocessing
import guard
import binance
import logging
import asyncio
from bot_commands import register_commands
from telethon import TelegramClient

BOT_TOKEN = os.environ.get('BOT_TOKEN')
ADMIN_ID = int(os.environ.get('ADMIN_ID'))

async def setup_bot():
    client = TelegramClient('bot', api_id=6, api_hash='eb06d4abfb49dc3eeb1aeb98ae0f581e')
    await client.start(bot_token=BOT_TOKEN)
    await register_commands(client, ADMIN_ID)
    await client.disconnect()

def run_setup_bot():
    asyncio.run(setup_bot())

def run_guard():
    while True:
        try:
            guard.run()
        except Exception as e:
            logging.error(f"Guard process crashed: {str(e)}")
            logging.info("Restarting Guard process...")

def run_binance():
    while True:
        try:
            binance.run()
        except Exception as e:
            logging.error(f"Binance process crashed: {str(e)}")
            logging.info("Restarting Binance process...")

if __name__ == '__main__':
    logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
    
    # 注册机器人命令
    run_setup_bot()
    
    # 创建两个进程分别运行 guard 和 binance 服务
    guard_process = multiprocessing.Process(target=run_guard)
    binance_process = multiprocessing.Process(target=run_binance)

    # 启动进程
    guard_process.start()
    binance_process.start()

    # 等待进程结束
    guard_process.join()
    binance_process.join()
