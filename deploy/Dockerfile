# Cherry 游戏服务器 Dockerfile
# 多阶段构建：编译 + 运行

# ==================== 编译阶段 ====================
FROM golang:1.23-alpine AS builder

# 安装编译依赖
RUN apk add --no-cache git

WORKDIR /build

# 复制 go.mod 和 go.sum 先下载依赖（利用缓存）
COPY go.mod go.sum ./
COPY vendor/ ./vendor/

# 复制源代码
COPY demo_cluster/ ./demo_cluster/
COPY config/ ./config/

# 编译游戏服务器
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -mod=vendor \
    -ldflags="-s -w" \
    -o game-server \
    ./demo_cluster/nodes/main.go

# ==================== 运行阶段 ====================
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从编译阶段复制二进制文件
COPY --from=builder /build/game-server .

# 创建日志目录
RUN mkdir -p /app/logs /app/config/data

# 设置权限
RUN chmod +x /app/game-server

# 默认入口
ENTRYPOINT ["./game-server"]
