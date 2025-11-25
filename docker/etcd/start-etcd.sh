#!/bin/bash

# Cherry框架etcd启动脚本
# 根据demo-cluster.json配置优化的etcd启动参数

set -e

echo "Starting etcd for Cherry Framework..."
echo "Configuration based on demo-cluster.json"

# 设置默认值
ETCD_NAME=${ETCD_NAME:-cherry-etcd-node}
ETCD_DATA_DIR=${ETCD_DATA_DIR:-/etcd-data}
ETCD_LISTEN_CLIENT_URLS=${ETCD_LISTEN_CLIENT_URLS:-http://0.0.0.0:2379}
ETCD_LISTEN_PEER_URLS=${ETCD_LISTEN_PEER_URLS:-http://0.0.0.0:2380}
ETCD_ADVERTISE_CLIENT_URLS=${ETCD_ADVERTISE_CLIENT_URLS:-http://dev.com:2379}
ETCD_INITIAL_ADVERTISE_PEER_URLS=${ETCD_INITIAL_ADVERTISE_PEER_URLS:-http://dev.com:2380}

# 创建数据目录
mkdir -p ${ETCD_DATA_DIR}

# 启动etcd
exec etcd \
  --name=${ETCD_NAME} \
  --data-dir=${ETCD_DATA_DIR} \
  --listen-client-urls=${ETCD_LISTEN_CLIENT_URLS} \
  --listen-peer-urls=${ETCD_LISTEN_PEER_URLS} \
  --advertise-client-urls=${ETCD_ADVERTISE_CLIENT_URLS} \
  --initial-advertise-peer-urls=${ETCD_INITIAL_ADVERTISE_PEER_URLS} \
  --initial-cluster=${ETCD_NAME}=${ETCD_INITIAL_ADVERTISE_PEER_URLS} \
  --initial-cluster-state=new \
  --initial-cluster-token=cherry-etcd-cluster \
  --auto-compaction-retention=1 \
  --quota-backend-bytes=4294967296 \
  --log-level=info \
  --logger=zap \
  --log-outputs=stderr