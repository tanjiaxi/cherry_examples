# 使用预编译的二进制文件
FROM debian:bookworm-slim

WORKDIR /app

# 安装运行时依赖
RUN apt-get update && apt-get install -y ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

# 复制预编译的二进制文件
COPY game-server .

# 复制配置文件
COPY ../config/ ./config/

# 暴露端口
EXPOSE 3010 3011 3012 3013 3014 3015

# 默认启动 web 节点
ENTRYPOINT ["./game-server"]
CMD ["web", "--path=./config/demo-cluster.json", "--node=gc-web-1"]
