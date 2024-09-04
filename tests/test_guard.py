import unittest
from unittest.mock import AsyncMock, patch
from guard import process_message, command_handler

class TestGuard(unittest.TestCase):
    @patch('guard.link_filter')
    async def test_process_message(self, mock_link_filter):
        mock_link_filter.should_filter.return_value = (True, [])
        event = AsyncMock()
        event.is_private = False
        event.sender_id = 12345  # 非管理员ID

        await process_message(event, AsyncMock())

        event.delete.assert_called_once()
        event.respond.assert_called_once_with("已撤回该消息。注:包含关键词或重复发送的非白名单链接会被自动撤回。")

    @patch('guard.handle_command')
    @patch('guard.link_filter')
    async def test_command_handler(self, mock_link_filter, mock_handle_command):
        event = AsyncMock()
        event.is_private = True
        event.sender_id = int(os.environ.get('ADMIN_ID'))
        event.raw_text = '/add keyword'

        await command_handler(event, mock_link_filter)

        mock_handle_command.assert_called_once()
        mock_link_filter.load_data_from_file.assert_called_once()

if __name__ == '__main__':
    unittest.main()
