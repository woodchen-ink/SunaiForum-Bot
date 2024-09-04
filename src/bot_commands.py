from telethon.tl.types import InputPeerUser
from telethon.tl.functions.bots import SetBotCommandsRequest
from telethon.tl.types import BotCommand

async def register_commands(client, admin_id):
    commands = [
        BotCommand('add', '添加新的关键词'),
        BotCommand('delete', '删除现有的关键词'),
        BotCommand('list', '列出所有当前的关键词'),
        BotCommand('addwhite', '添加域名到白名单'),
        BotCommand('delwhite', '从白名单移除域名'),
        BotCommand('listwhite', '列出白名单域名'),
    ]
    
    try:
        await client(SetBotCommandsRequest(
            commands=commands,
            scope=InputPeerUser(admin_id, 0),
            lang_code=''
        ))
        print("Bot commands registered successfully.")
    except Exception as e:
        print(f"Failed to register bot commands: {str(e)}")

__all__ = ['register_commands']
