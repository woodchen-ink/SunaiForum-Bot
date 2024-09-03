# Q58-Telegram-Bot

## 示例

![image](https://github.com/user-attachments/assets/b5651dd9-495f-4a65-a248-610956c4a6c1)


## 项目简介

这个项目主要功能：

1. TeleGuard：一个 Telegram 机器人，用于管理群组中的关键词并自动删除包含这些关键词的消息。
2. 币安价格更新器：定期获取并发送指定加密货币的价格信息。

这些功能被整合到一个 Docker 容器中，可以同时运行。

## 功能特点

### TeleGuard
- 自动删除包含指定关键词的消息
- 支持通过命令添加、删除和列出关键词
- 只有管理员可以管理关键词列表

### 币安价格更新器
- 定期获取指定加密货币的价格信息
- 发送详细的价格更新，包括当前价格、24小时变化、高低点等
- 可自定义更新频率和货币对

## 安装与配置

1. 克隆此仓库到本地
2. 确保已安装 Docker 和 Docker Compose
3. 使用 `docker-compose.yml` 文件构建和启动容器

## 使用方法

1. 构建并启动 Docker 容器：
   ```
   docker-compose up -d 
   ```

2. 查看日志：
   ```
   docker-compose logs -f
   ```

3. TeleGuard 命令：
   - `/add 关键词`：添加新的关键词
   - `/delete 关键词`：删除现有的关键词
   - `/list`：列出所有当前的关键词

## 注意事项

- 确保 Telegram 机器人已被添加到目标群组，并被赋予管理员权限
- 币安 API 可能有请求限制，请注意控制请求频率
- 定期检查日志以确保服务正常运行

## 贡献

欢迎提交 Issues 和 Pull Requests 来帮助改进这个项目。

## 许可证

[MIT License](LICENSE)
