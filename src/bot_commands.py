import os
from telethon.tl.types import InputPeerUser
from telethon.tl.functions.bots import SetBotCommandsRequest
from telethon.tl.types import BotCommand
from link_filter import LinkFilter

KEYWORDS_FILE = '/app/data/keywords.json'
WHITELIST_FILE = '/app/data/whitelist.json'
ADMIN_ID = int(os.environ.get('ADMIN_ID'))

# 创建 LinkFilter 实例
link_filter = LinkFilter(KEYWORDS_FILE, WHITELIST_FILE)

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

async def handle_command(event, client):
    sender = await event.get_sender()
    if sender.id != ADMIN_ID:
        return

    link_filter.load_data_from_file()  # 在处理命令前重新加载数据
    
    command, *args = event.message.text.split()
    command = command.lower()

    if command in ['/add', '/delete', '/list']:
        await handle_keyword_command(event, command, args)
    elif command in ['/addwhite', '/delwhite', '/listwhite']:
        await handle_whitelist_command(event, command, args)

async def handle_keyword_command(event, command, args):
    if command == '/list':
        link_filter.load_data_from_file()  # 确保使用最新数据
        keywords = link_filter.keywords
        await event.reply("当前关键词列表：\n" + "\n".join(keywords) if keywords else "关键词列表为空。")
    elif command == '/add' and args:
        keyword = ' '.join(args)
        if keyword not in link_filter.keywords:
            link_filter.add_keyword(keyword)
            await event.reply(f"关键词 '{keyword}' 已添加。")
        else:
            await event.reply(f"关键词 '{keyword}' 已存在。")
    elif command == '/delete' and args:
        keyword = ' '.join(args)
        if link_filter.remove_keyword(keyword):
            await event.reply(f"关键词 '{keyword}' 已删除。")
        else:
            # 如果没有精确匹配，尝试查找部分匹配的关键词
            similar_keywords = [k for k in link_filter.keywords if keyword.lower() in k.lower()]
            if similar_keywords:
                await event.reply(f"未找到精确匹配的关键词 '{keyword}'。\n\n以下是相似的关键词：\n" + "\n".join(similar_keywords))
            else:
                await event.reply(f"关键词 '{keyword}' 不存在。")
    else:
        await event.reply("无效的命令或参数。")


async def handle_whitelist_command(event, command, args):
    if command == '/listwhite':
        link_filter.load_data_from_file()  # 确保使用最新数据
        whitelist = link_filter.whitelist
        await event.reply("白名单域名列表：\n" + "\n".join(whitelist) if whitelist else "白名单为空。")
    elif command == '/addwhite' and args:
        domain = args[0].lower()
        if domain not in link_filter.whitelist:
            link_filter.whitelist.append(domain)
            link_filter.save_whitelist()
            link_filter.load_data_from_file()  # 重新加载以确保数据同步
            await event.reply(f"域名 '{domain}' 已添加到白名单。")
        else:
            await event.reply(f"域名 '{domain}' 已在白名单中。")
    elif command == '/delwhite' and args:
        domain = args[0].lower()
        if domain in link_filter.whitelist:
            link_filter.whitelist.remove(domain)
            link_filter.save_whitelist()
            link_filter.load_data_from_file()  # 重新加载以确保数据同步
            await event.reply(f"域名 '{domain}' 已从白名单中删除。")
        else:
            await event.reply(f"域名 '{domain}' 不在白名单中。")
    else:
        await event.reply("无效的命令或参数。")


def get_keywords():
    link_filter.load_data_from_file()
    return link_filter.keywords

def get_whitelist():
    link_filter.load_data_from_file()
    return link_filter.whitelist

__all__ = ['handle_command', 'get_keywords', 'get_whitelist', 'register_commands']
