# 使用官方 Go 镜像作为构建环境
FROM golang:1.22 AS builder

# 设置工作目录
WORKDIR /app

# 复制 go mod 和 sum 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .

# 使用轻量级的 alpine 镜像作为运行环境
FROM alpine:latest  

# 安装 ca-certificates
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段复制编译好的应用
COPY --from=builder /app/main .

# 设置环境变量
ENV BOT_TOKEN=""
ENV ADMIN_ID=""
ENV SYMBOLS=""
ENV DEBUG_MODE="false"

# 创建数据目录
RUN mkdir -p /app/data

# 暴露端口（如果你的应用需要的话）
# EXPOSE 8080

# 运行应用
CMD ["./main"]
