# Cherry框架专用的etcd Docker镜像
FROM quay.io/coreos/etcd:v3.5.9

# 设置工作目录
WORKDIR /opt/etcd

# 创建必要的目录
RUN mkdir -p /etcd-data /var/log/etcd

# 复制启动脚本
COPY start-etcd.sh /opt/etcd/start-etcd.sh
RUN chmod +x /opt/etcd/start-etcd.sh

# 设置环境变量
ENV ETCD_DATA_DIR=/etcd-data
ENV ETCD_LISTEN_CLIENT_URLS=http://0.0.0.0:2379
ENV ETCD_LISTEN_PEER_URLS=http://0.0.0.0:2380

# 暴露端口
EXPOSE 2379 2380

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD etcdctl endpoint health

# 启动命令
CMD ["/opt/etcd/start-etcd.sh"]