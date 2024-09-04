import re
import json
import tldextract
import urllib.parse
import logging

logger = logging.getLogger('TeleGuard.LinkFilter')


class LinkFilter:
    def __init__(self, keywords_file, whitelist_file):
        self.keywords_file = keywords_file
        self.whitelist_file = whitelist_file
        self.keywords = []
        self.whitelist = []
        self.load_data_from_file()
        
        self.link_pattern = re.compile(r'''
            \b
            (?:
                (?:https?://)?                     # http:// or https:// (optional)
                (?:(?:www\.)?                      # www. (optional)
                (?:[a-zA-Z0-9-]+\.)+            # domain
                [a-zA-Z]{2,}                    # TLD
                |                               # or
                (?:t\.me|telegram\.me)          # Telegram links
                )
                (?:/[^\s]*)?                       # optional path and query string
            )
            \b
        ''', re.VERBOSE | re.IGNORECASE)


    def load_json(self, file_path):
        try:
            with open(file_path, 'r') as f:
                return json.load(f)
        except FileNotFoundError:
            return []

    def save_json(self, file_path, data):
        with open(file_path, 'w') as f:
            json.dump(data, f)

    def save_keywords(self):
        self.save_json(self.keywords_file, self.keywords)

    def save_whitelist(self):
        self.save_json(self.whitelist_file, self.whitelist)

    def load_data_from_file(self):
        self.keywords = self.load_json(self.keywords_file)
        self.whitelist = self.load_json(self.whitelist_file)
        logger.info(f"Reloaded {len(self.keywords)} keywords and {len(self.whitelist)} whitelist entries")

    def normalize_link(self, link):
        link = re.sub(r'^https?://', '', link)
        parsed = urllib.parse.urlparse(f"http://{link}")
        normalized = urllib.parse.urlunparse(('', parsed.netloc, parsed.path, parsed.params, parsed.query, ''))
        result = normalized.rstrip('/')
        logger.debug(f"Normalized link: {link} -> {result}")
        return result

    def is_whitelisted(self, link):
        extracted = tldextract.extract(link)
        domain = f"{extracted.domain}.{extracted.suffix}"
        result = domain in self.whitelist
        logger.debug(f"Whitelist check for {link}: {'Passed' if result else 'Failed'}")
        return result

    def add_keyword(self, keyword):
        if self.link_pattern.match(keyword):
            keyword = self.normalize_link(keyword)
        if keyword not in self.keywords:
            self.keywords.append(keyword)
            self.save_keywords()
            logger.info(f"New keyword added: {keyword}")
            self.load_data_from_file()  # 重新加载文件
        else:
            logger.debug(f"Keyword already exists: {keyword}")

    def remove_keyword(self, keyword):
        if keyword in self.keywords:
            self.keywords.remove(keyword)
            self.save_keywords()
            self.load_data_from_file()  # 重新加载以确保数据同步
            return True
        return False


    def should_filter(self, text):
        logger.debug(f"Checking text: {text}")
        if any(keyword.lower() in text.lower() for keyword in self.keywords):
            logger.info(f"Text contains keyword: {text}")
            return True, []

        links = self.link_pattern.findall(text)
        logger.debug(f"Found links: {links}")
        new_non_whitelisted_links = []
        for link in links:
            normalized_link = self.normalize_link(link)
            if not self.is_whitelisted(normalized_link):
                logger.debug(f"Link not whitelisted: {normalized_link}")
                if normalized_link not in self.keywords:
                    new_non_whitelisted_links.append(normalized_link)
                    self.add_keyword(normalized_link)
                else:
                    logger.info(f"Existing keyword found: {normalized_link}")
                    return True, []
        
        if new_non_whitelisted_links:
            logger.info(f"New non-whitelisted links found: {new_non_whitelisted_links}")
        return False, new_non_whitelisted_links
    
    async def handle_keyword_command(self, event, command, args):
        if command == '/list':
            self.load_data_from_file()
            keywords = self.keywords
            await event.reply("当前关键词列表：\n" + "\n".join(keywords) if keywords else "关键词列表为空。")
        elif command == '/add' and args:
            keyword = ' '.join(args)
            if keyword not in self.keywords:
                self.add_keyword(keyword)
                await event.reply(f"关键词 '{keyword}' 已添加。")
            else:
                await event.reply(f"关键词 '{keyword}' 已存在。")
        elif command == '/delete' and args:
            keyword = ' '.join(args)
            if self.remove_keyword(keyword):
                await event.reply(f"关键词 '{keyword}' 已删除。")
            else:
                similar_keywords = [k for k in self.keywords if keyword.lower() in k.lower()]
                if similar_keywords:
                    await event.reply(f"未找到精确匹配的关键词 '{keyword}'。\n\n以下是相似的关键词：\n" + "\n".join(similar_keywords))
                else:
                    await event.reply(f"关键词 '{keyword}' 不存在。")
        else:
            await event.reply("无效的命令或参数。")

    async def handle_whitelist_command(self, event, command, args):
        if command == '/listwhite':
            self.load_data_from_file()
            whitelist = self.whitelist
            await event.reply("白名单域名列表：\n" + "\n".join(whitelist) if whitelist else "白名单为空。")
        elif command == '/addwhite' and args:
            domain = args[0].lower()
            if domain not in self.whitelist:
                self.whitelist.append(domain)
                self.save_whitelist()
                self.load_data_from_file()
                await event.reply(f"域名 '{domain}' 已添加到白名单。")
            else:
                await event.reply(f"域名 '{domain}' 已在白名单中。")
        elif command == '/delwhite' and args:
            domain = args[0].lower()
            if domain in self.whitelist:
                self.whitelist.remove(domain)
                self.save_whitelist()
                self.load_data_from_file()
                await event.reply(f"域名 '{domain}' 已从白名单中删除。")
            else:
                await event.reply(f"域名 '{domain}' 不在白名单中。")
        else:
            await event.reply("无效的命令或参数。")