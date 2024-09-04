import re
import json
import tldextract
import urllib.parse

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


    def normalize_link(self, link):
        # 解析链接
        parsed = urllib.parse.urlparse(link)
        
        # 如果没有 scheme，添加 'https://'
        if not parsed.scheme:
            link = 'https://' + link
            parsed = urllib.parse.urlparse(link)
        
        # 重新组合链接，去除查询参数
        normalized = urllib.parse.urlunparse((
            parsed.scheme,
            parsed.netloc,
            parsed.path,
            '',
            '',
            ''
        ))
        
        return normalized.rstrip('/')  # 移除尾部的斜杠
    def add_keyword(self, keyword):
        if self.link_pattern.match(keyword):
            keyword = self.normalize_link(keyword)
        if keyword not in self.keywords:
            self.keywords.append(keyword)
            self.save_keywords()

    def remove_keyword(self, keyword):
        if keyword in self.keywords:
            self.keywords.remove(keyword)
            self.save_keywords()
            return True
        return False


    def should_filter(self, text):
        # 检查是否包含关键词
        if any(keyword.lower() in text.lower() for keyword in self.keywords if not self.link_pattern.match(keyword)):
            return True, []

        links = self.link_pattern.findall(text)
        new_non_whitelisted_links = []
        for link in links:
            normalized_link = self.normalize_link(link)
            if not self.is_whitelisted(normalized_link):
                if normalized_link not in self.keywords:
                    new_non_whitelisted_links.append(normalized_link)
                    self.add_keyword(normalized_link)
                else:
                    return True, []  # 如果找到已存在的非白名单链接，应该过滤
        
        return False, new_non_whitelisted_links

    def reload_keywords(self):
        self.keywords = self.load_json(self.keywords_file)

    def reload_whitelist(self):
        self.whitelist = self.load_json(self.whitelist_file)
    
    def save_whitelist(self):
        with open(self.whitelist_file, 'w') as f:
            json.dump(self.whitelist, f)
