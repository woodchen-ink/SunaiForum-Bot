from telethon.tl.types import InputPeerUser
from telethon.tl.functions.bots import SetBotCommandsRequest
from telethon.tl.types import BotCommand
import logging

async def register_commands(client, admin_id):
    commands = [
        # TeleGuard 命令
        BotCommand('add', '添加新的关键词'),
        BotCommand('delete', '删除现有的关键词'),
        BotCommand('list', '列出所有当前的关键词'),
        
        # 这里可以添加其他功能的命令
        # 例如：BotCommand('price', '获取当前价格'),
    ]
    
    try:
        await client(SetBotCommandsRequest(
            commands=commands,
            scope=InputPeerUser(admin_id, 0),
            lang_code=''
        ))
        logging.info("Bot commands registered successfully.")
    except Exception as e:
        logging.error(f"Failed to register bot commands: {str(e)}")

# 如果有其他功能需要注册命令，可以在这里添加新的函数
