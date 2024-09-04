import os
import json
from telethon.tl.types import InputPeerUser
from telethon.tl.functions.bots import SetBotCommandsRequest
from telethon.tl.types import BotCommand

KEYWORDS_FILE = '/app/data/keywords.json'
WHITELIST_FILE = '/app/data/whitelist.json'
ADMIN_ID = int(os.environ.get('ADMIN_ID'))

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
        
def load_json(file_path):
    try:
        with open(file_path, 'r') as f:
            return json.load(f)
    except FileNotFoundError:
        return []

def save_json(file_path, data):
    with open(file_path, 'w') as f:
        json.dump(data, f)

async def handle_command(event, client):
    sender = await event.get_sender()
    if sender.id != ADMIN_ID:
        return

    command, *args = event.message.text.split()
    command = command.lower()

    if command in ['/add', '/delete', '/list']:
        await handle_keyword_command(event, command, args)
    elif command in ['/addwhite', '/delwhite', '/listwhite']:
        await handle_whitelist_command(event, command, args)

async def handle_keyword_command(event, command, args):
    keywords = load_json(KEYWORDS_FILE)
    
    if command == '/list':
        await event.reply("当前关键词列表：\n" + "\n".join(keywords) if keywords else "关键词列表为空。")
    elif command == '/add' and args:
        keyword = args[0].lower()
        if keyword not in keywords:
            keywords.append(keyword)
            save_json(KEYWORDS_FILE, keywords)
            await event.reply(f"关键词 '{keyword}' 已添加。")
        else:
            await event.reply(f"关键词 '{keyword}' 已存在。")
    elif command == '/delete' and args:
        keyword = args[0].lower()
        if keyword in keywords:
            keywords.remove(keyword)
            save_json(KEYWORDS_FILE, keywords)
            await event.reply(f"关键词 '{keyword}' 已删除。")
        else:
            await event.reply(f"关键词 '{keyword}' 不存在。")
    else:
        await event.reply("无效的命令或参数。")

async def handle_whitelist_command(event, command, args):
    whitelist = load_json(WHITELIST_FILE)
    
    if command == '/listwhite':
        await event.reply("白名单域名列表：\n" + "\n".join(whitelist) if whitelist else "白名单为空。")
    elif command == '/addwhite' and args:
        domain = args[0].lower()
        if domain not in whitelist:
            whitelist.append(domain)
            save_json(WHITELIST_FILE, whitelist)
            await event.reply(f"域名 '{domain}' 已添加到白名单。")
        else:
            await event.reply(f"域名 '{domain}' 已在白名单中。")
    elif command == '/delwhite' and args:
        domain = args[0].lower()
        if domain in whitelist:
            whitelist.remove(domain)
            save_json(WHITELIST_FILE, whitelist)
            await event.reply(f"域名 '{domain}' 已从白名单中删除。")
        else:
            await event.reply(f"域名 '{domain}' 不在白名单中。")
    else:
        await event.reply("无效的命令或参数。")

def get_keywords():
    return load_json(KEYWORDS_FILE)

def get_whitelist():
    return load_json(WHITELIST_FILE)



__all__ = ['handle_command', 'get_keywords', 'get_whitelist', 'register_commands']

