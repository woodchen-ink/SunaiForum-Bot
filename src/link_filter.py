import re
import urllib.parse
import logging
from database import Database
from functions import send_long_message

logger = logging.getLogger("TeleGuard.LinkFilter")

class LinkFilter:
    def __init__(self, db_file):
        self.db = Database(db_file)
        self.load_data_from_file()

        self.link_pattern = re.compile(
            r"""
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
        """,
            re.VERBOSE | re.IGNORECASE,
        )


    def load_data_from_file(self):
        self.keywords = self.db.get_all_keywords()
        self.whitelist = self.db.get_all_whitelist()
        logger.info(
            f"Loaded {len(self.keywords)} keywords and {len(self.whitelist)} whitelist entries from database"
        )

    def normalize_link(self, link):
        link = re.sub(r"^https?://", "", link)
        link = link.lstrip("/")
        parsed = urllib.parse.urlparse(f"http://{link}")
        normalized = urllib.parse.urlunparse(
            ("", parsed.netloc, parsed.path, parsed.params, parsed.query, "")
        )
        result = normalized.rstrip("/")
        logger.debug(f"Normalized link: {link} -> {result}")
        return result

    def extract_domain(self, url):
        parsed = urllib.parse.urlparse(url)
        domain = parsed.netloc or parsed.path
        domain = domain.split(':')[0]  # Remove port if present
        parts = domain.split('.')
        if len(parts) > 2:
            domain = '.'.join(parts[-2:])
        return domain.lower()
    def is_whitelisted(self, link):
        domain = self.extract_domain(link)
        result = domain in self.whitelist
        logger.debug(f"Whitelist check for {link}: {'Passed' if result else 'Failed'}")
        return result

    def add_keyword(self, keyword):
        if self.link_pattern.match(keyword):
            keyword = self.normalize_link(keyword)
        keyword = keyword.lstrip("/")
        if keyword not in self.keywords:
            self.db.add_keyword(keyword)
            logger.info(f"New keyword added: {keyword}")
            self.load_data_from_file()
        else:
            logger.debug(f"Keyword already exists: {keyword}")

    def remove_keyword(self, keyword):
        if keyword in self.keywords:
            self.db.remove_keyword(keyword)
            self.load_data_from_file()
            return True
        return False

    def remove_keywords_containing(self, substring):
        removed_keywords = [kw for kw in self.keywords if substring.lower() in kw.lower()]
        for keyword in removed_keywords:
            self.db.remove_keyword(keyword)
        self.load_data_from_file()
        return removed_keywords

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
            normalized_link = normalized_link.lstrip("/")
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
        if command == "/list":
            self.load_data_from_file()
            keywords = self.keywords
            if not keywords:
                await event.reply("关键词列表为空。")
            else:
                await send_long_message(event, "当前关键词列表：", keywords)
        elif command == "/add" and args:
            keyword = " ".join(args)
            if keyword not in self.keywords:
                self.add_keyword(keyword)
                await event.reply(f"关键词 '{keyword}' 已添加。")
            else:
                await event.reply(f"关键词 '{keyword}' 已存在。")
        elif command == "/delete" and args:
            keyword = " ".join(args)
            if self.remove_keyword(keyword):
                await event.reply(f"关键词 '{keyword}' 已删除。")
            else:
                similar_keywords = [k for k in self.keywords if keyword.lower() in k.lower()]
                if similar_keywords:
                    await send_long_message(
                        event,
                        f"未找到精确匹配的关键词 '{keyword}'。\n\n以下是相似的关键词：",
                        similar_keywords,
                    )
                else:
                    await event.reply(f"关键词 '{keyword}' 不存在。")
        elif command == "/deletecontaining" and args:
            substring = " ".join(args)
            removed_keywords = self.remove_keywords_containing(substring)
            if removed_keywords:
                await send_long_message(
                    event, f"已删除包含 '{substring}' 的以下关键词：", removed_keywords
                )
            else:
                await event.reply(f"没有找到包含 '{substring}' 的关键词。")
        else:
            await event.reply("无效的命令或参数。")

    async def handle_whitelist_command(self, event, command, args):
        if command == "/listwhite":
            self.load_data_from_file()
            whitelist = self.whitelist
            await event.reply(
                "白名单域名列表：\n" + "\n".join(whitelist)
                if whitelist
                else "白名单为空。"
            )
        elif command == "/addwhite" and args:
            domain = args[0].lower()
            if domain not in self.whitelist:
                self.db.add_whitelist(domain)
                self.load_data_from_file()
                await event.reply(f"域名 '{domain}' 已添加到白名单。")
            else:
                await event.reply(f"域名 '{domain}' 已在白名单中。")
        elif command == "/delwhite" and args:
            domain = args[0].lower()
            if domain in self.whitelist:
                self.db.remove_whitelist(domain)
                self.load_data_from_file()
                await event.reply(f"域名 '{domain}' 已从白名单中删除。")
            else:
                await event.reply(f"域名 '{domain}' 不在白名单中。")
        else:
            await event.reply("无效的命令或参数。")
