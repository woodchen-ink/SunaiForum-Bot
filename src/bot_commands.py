import os
from telethon import events
from telethon.tl.types import InputPeerUser
from telethon.tl.functions.bots import SetBotCommandsRequest
from telethon.tl.types import BotCommand
import logging
import json

__all__ = ['register_commands', 'handle_command', 'get_keywords', 'get_whitelist']


KEYWORDS_FILE = '/app/data/keywords.json'
WHITELIST_FILE = '/app/data/whitelist.json'
ADMIN_ID = int(os.environ.get('ADMIN_ID'))

def load_json(file_path):
    try:
        with open(file_path, 'r') as f:
            return json.load(f)
    except FileNotFoundError:
        return []

def save_json(file_path, data):
    with open(file_path, 'w') as f:
        json.dump(data, f)

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
        logging.info("Bot commands registered successfully.")
    except Exception as e:
        logging.error(f"Failed to register bot commands: {str(e)}")

async def handle_keyword_command(event, client):
    sender = await event.get_sender()
    if sender.id != ADMIN_ID:
        return

    command = event.message.text.split(maxsplit=1)
    
    if command[0].lower() == '/list':
        await execute_keyword_command(event, '/list', '')
    elif len(command) > 1:
        await execute_keyword_command(event, command[0], command[1])
    else:
        await event.reply(f"请输入你要{command[0][1:]}的关键词：")
        async with client.conversation(sender) as conv:
            response = await conv.get_response()
            await execute_keyword_command(event, command[0], response.text)

async def execute_keyword_command(event, command, keyword):
    keywords = load_json(KEYWORDS_FILE)
    
    if command.lower() == '/list':
        if keywords:
            await event.reply(f"当前关键词和语句列表：\n" + "\n".join(keywords))
        else:
            await event.reply("关键词列表为空。")
        return
    
    if command.lower() == '/add':
        if keyword.lower() not in keywords:
            keywords.append(keyword.lower())
            save_json(KEYWORDS_FILE, keywords)
            await event.reply(f"关键词或语句 '{keyword}' 已添加到列表中。")
        else:
            await event.reply(f"关键词或语句 '{keyword}' 已经在列表中。")
    
    elif command.lower() == '/delete':
        if keyword.lower() in keywords:
            keywords.remove(keyword.lower())
            save_json(KEYWORDS_FILE, keywords)
            await event.reply(f"关键词或语句 '{keyword}' 已从列表中删除。")
        else:
            await event.reply(f"关键词或语句 '{keyword}' 不在列表中。")

async def handle_whitelist_command(event, client):
    sender = await event.get_sender()
    if sender.id != ADMIN_ID:
        return

    command = event.message.text.split(maxsplit=1)
    
    if command[0].lower() == '/listwhite':
        await execute_whitelist_command(event, '/listwhite', '')
    elif len(command) > 1:
        await execute_whitelist_command(event, command[0], command[1])
    else:
        await event.reply(f"请输入你要{command[0][1:]}的域名：")
        async with client.conversation(sender) as conv:
            response = await conv.get_response()
            await execute_whitelist_command(event, command[0], response.text)

async def execute_whitelist_command(event, command, domain):
    whitelist = load_json(WHITELIST_FILE)
    
    if command.lower() == '/listwhite':
        if whitelist:
            await event.reply("白名单域名列表：\n" + "\n".join(whitelist))
        else:
            await event.reply("白名单为空。")
        return
    
    if command.lower() == '/addwhite':
        if domain.lower() not in whitelist:
            whitelist.append(domain.lower())
            save_json(WHITELIST_FILE, whitelist)
            await event.reply(f"域名 '{domain}' 已添加到白名单。")
        else:
            await event.reply(f"域名 '{domain}' 已经在白名单中。")
    
    elif command.lower() == '/delwhite':
        if domain.lower() in whitelist:
            whitelist.remove(domain.lower())
            save_json(WHITELIST_FILE, whitelist)
            await event.reply(f"域名 '{domain}' 已从白名单中移除。")
        else:
            await event.reply(f"域名 '{domain}' 不在白名单中。")

def get_keywords():
    return load_json(KEYWORDS_FILE)

def get_whitelist():
    return load_json(WHITELIST_FILE)

async def handle_command(event, client):
    sender = await event.get_sender()
    if sender.id != ADMIN_ID:
        return

    command = event.message.text.split(maxsplit=1)
    
    if command[0].lower() in ['/add', '/delete', '/list']:
        await handle_keyword_command(event, client)
    elif command[0].lower() in ['/addwhite', '/delwhite', '/listwhite']:
        await handle_whitelist_command(event, client)


