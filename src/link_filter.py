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
        domain = f"{extracted.domain}.{extracted.suffix}"
        return domain in self.whitelist

    def add_keyword(self, link):
        if link not in self.keywords:
            self.keywords.append(link)
            self.save_keywords()

    def should_filter(self, text):
        links = self.link_pattern.findall(text)
        for link in links:
            if not self.is_whitelisted(link):
                if link in self.keywords:
                    return True
                else:
                    self.add_keyword(link)
        return False

    def reload_keywords(self):
        self.keywords = self.load_json(self.keywords_file)

    def reload_whitelist(self):
        self.whitelist = self.load_json(self.whitelist_file)
