# 长消息发送截断成多个消息
async def send_long_message(event, prefix, items):
    message = prefix + "\n"
    for i, item in enumerate(items, 1):
        if len(message) + len(item) > 4000:  # 留一些余地
            await event.reply(message)
            message = ""
        message += f"{i}. {item}\n"
    if message:
        await event.reply(message)
