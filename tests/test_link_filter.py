import unittest
import tempfile
import json
from link_filter import LinkFilter

class TestLinkFilter(unittest.TestCase):
    def setUp(self):
        # 创建临时文件作为关键词和白名单文件
        self.keywords_file = tempfile.NamedTemporaryFile(mode='w+', delete=False)
        self.whitelist_file = tempfile.NamedTemporaryFile(mode='w+', delete=False)
        
        # 写入一些初始数据
        json.dump(['example.com'], self.keywords_file)
        json.dump(['google.com'], self.whitelist_file)
        
        self.keywords_file.close()
        self.whitelist_file.close()
        
        self.link_filter = LinkFilter(self.keywords_file.name, self.whitelist_file.name)

    def test_normalize_link(self):
        self.assertEqual(self.link_filter.normalize_link('https://www.example.com'), 'www.example.com')
        self.assertEqual(self.link_filter.normalize_link('http://example.com'), 'example.com')

    def test_is_whitelisted(self):
        self.assertTrue(self.link_filter.is_whitelisted('https://www.google.com'))
        self.assertFalse(self.link_filter.is_whitelisted('https://www.example.com'))

    def test_should_filter(self):
        should_filter, new_links = self.link_filter.should_filter('Check out https://www.example.com')
        self.assertTrue(should_filter)
        self.assertEqual(new_links, [])

        should_filter, new_links = self.link_filter.should_filter('Check out https://www.newsite.com')
        self.assertFalse(should_filter)
        self.assertEqual(new_links, ['www.newsite.com'])

    def tearDown(self):
        # 删除临时文件
        import os
        os.unlink(self.keywords_file.name)
        os.unlink(self.whitelist_file.name)

if __name__ == '__main__':
    unittest.main()
