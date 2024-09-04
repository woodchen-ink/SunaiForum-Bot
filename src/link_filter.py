import re
import json
import tldextract

class LinkFilter:
    def __init__(self, keywords_file, whitelist_file):
        self.keywords_file = keywords_file
        self.whitelist_file = whitelist_file
        self.keywords = self.load_json(keywords_file)
        self.whitelist = self.load_json(whitelist_file)
        
        # 正则表达式匹配各种链接格式
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
                (?:/[^\s]*)?                       # optional path
            )
            \b
        ''', re.VERBOSE | re.IGNORECASE)

    def load_json(self, file_path):
        try:
            with open(file_path, 'r') as f:
                return json.load(f)
        except FileNotFoundError:
            return []

    def save_keywords(self):
        with open(self.keywords_file, 'w') as f:
            json.dump(self.keywords, f)

    def is_whitelisted(self, link):
        extracted = tldextract.extract(link)
        full_domain = '.'.join(part for part in [extracted.subdomain, extracted.domain, extracted.suffix] if part)
        main_domain = f"{extracted.domain}.{extracted.suffix}"
        
        # 检查完整域名（包括子域名）
        if full_domain in self.whitelist:
            return True
        
        # 检查主域名
        if main_domain in self.whitelist:
            return True
        
        # 检查是否有通配符匹配
        wildcard_domain = f"*.{main_domain}"
        if wildcard_domain in self.whitelist:
            return True
        
        return False


    def add_keyword(self, link):
        if link not in self.keywords:
            self.keywords.append(link)
            self.save_keywords()

    def should_filter(self, text):
        links = self.link_pattern.findall(text)
        new_non_whitelisted_links = []
        for link in links:
            if not self.is_whitelisted(link):
                if link not in self.keywords:
                    new_non_whitelisted_links.append(link)
                    self.add_keyword(link)
        return new_non_whitelisted_links

    def reload_keywords(self):
        self.keywords = self.load_json(self.keywords_file)

    def reload_whitelist(self):
        self.whitelist = self.load_json(self.whitelist_file)
