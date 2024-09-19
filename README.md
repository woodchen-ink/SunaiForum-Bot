# Q58-Telegram-Bot

## 示例

![image](https://github.com/user-attachments/assets/b5651dd9-495f-4a65-a248-610956c4a6c1)
![image](https://github.com/user-attachments/assets/6188410f-3c67-49d1-80a8-6ca28541c8c0)
![image](https://github.com/user-attachments/assets/57017af9-7ec1-41c6-b287-a8b2decd60f8)


## 项目功能


### TeleGuard
- 自动删除包含指定关键词的消息
- 支持通过命令添加、删除和列出关键词
- 只有管理员可以管理关键词列表

### 币安价格更新器
- 定期获取指定加密货币的价格信息
- 发送详细的价格更新，包括当前价格、24小时变化、高低点等
- 可自定义货币对, 更新频率可自行在代码里修改

### 链接拦截
- 新增: 当非管理员时, 才会进行链接拦截
- 非白名单域名链接, 在发送第二次会被拦截撤回

### 白名单域名
- 当用户发送链接, 属于白名单域名, 则不进行操作. 如果不属于白名单域名, 则会第一次允许发送, 第二次进行撤回操作.
- 会匹配链接中的域名, 包括二级域名和三级域名
- 例如，如果白名单中有 "example.com"，它将匹配 "example.com"、"sub.example.com" 和 "sub.sub.example.com"。
- 同时，如果白名单中有 "sub.example.com"，它将匹配 "sub.example.com" 和 "subsub.sub.example.com"，但不会匹配 "example.com" 或 "othersub.example.com"。

### 提示词自动回复
- 当用户发送包含特定关键词的消息时，机器人将自动回复提示词。
- 管理员通过`/prompt`进行设置, 支持添加, 删除, 列出.

### 群组快捷管理
- 管理员可以对成员消息回复`/ban`, 会进行以下处理: 
  1. 将成员消息撤回, 无限期封禁成员, 并发送封禁通知
  2. 在3分钟后, 撤回管理员指令消息和机器人的封禁通知


## 安装与配置

1. 确保服务器已安装 Docker 和 Docker Compose
2. 使用 `docker-compose.yml` 文件构建和启动容器

## 使用方法

构建并启动 Docker 容器：
```
docker-compose up -d 
```

## 注意事项

- 确保 Telegram 机器人已被添加到目标群组，并被赋予管理员权限
- 定期检查日志以确保服务正常运行

## 项目结构
```
Q58Bot/
│
├── core/
│   ├── bot_commands.go # 注册机器人命令
│   ├── database.go # 数据库操作
│   ├── init.go # 初始化全局变量
│   └── ratelimiter.go # 限速器
│
├── service/
│   ├── binance/
│   │   └── binance.go # 获取币安价格信息
│   │   
|   |── group_member_management/
|   |  └── group_member_management.go # 对群组进行管理
│   │  
│   ├── link_filter/
│   │   └── link_filter.go # 链接过滤器
│   │
│   ├── prompt_reply/
│   |   └── prompt_reply.go # 提示词自动回复
│   │
│   └── message_handler.go # 消息处理器
│
├── docker-compose.yml
├── Dockerfile.multi
├── go.mod
├── go.sum
├── main.go # 入口文件
└── README.md
```
## 设计规范

> 自己记录

1. 关于数据库的操作, 需要在运行时进行一次加载数据
2. 任何操作需要添加相关日志, 日志需要包含时间戳
3. 要使用全局的数据库实例`core.DB`, 相关代码如下:
   - `init.go`里
      ``` go
      // 初始化数据库
      DB_FILE = filepath.Join("/app/data", "q58.db")
      var err error
      DB, err = NewDatabase()
      if err != nil {
         return fmt.Errorf("初始化数据库失败: %v", err)
      }
      ```

## 贡献

欢迎提交 Issues 和 Pull Requests 来帮助改进这个项目。

## 许可证

[MIT License](LICENSE)