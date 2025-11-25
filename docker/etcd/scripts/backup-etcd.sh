#!/bin/bash

# Cherry框架etcd备份脚本

set -e

# 配置
BACKUP_DIR="./backups"
ETCD_ENDPOINTS="http://dev.com:2379"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="cherry-etcd-backup-${DATE}.db"

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}🔄 Cherry框架 etcd 备份脚本${NC}"
echo "================================"

# 创建备份目录
mkdir -p ${BACKUP_DIR}

# 设置etcd环境变量
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=${ETCD_ENDPOINTS}

echo -e "${YELLOW}📦 创建etcd快照...${NC}"
etcdctl snapshot save ${BACKUP_DIR}/${BACKUP_FILE}

echo -e "${YELLOW}✅ 验证快照完整性...${NC}"
etcdctl snapshot status ${BACKUP_DIR}/${BACKUP_FILE} --write-out=table

# 压缩备份文件
echo -e "${YELLOW}🗜️  压缩备份文件...${NC}"
gzip ${BACKUP_DIR}/${BACKUP_FILE}

# 显示备份信息
BACKUP_SIZE=$(du -h ${BACKUP_DIR}/${BACKUP_FILE}.gz | cut -f1)
echo -e "${GREEN}✅ 备份完成！${NC}"
echo "备份文件: ${BACKUP_DIR}/${BACKUP_FILE}.gz"
echo "文件大小: ${BACKUP_SIZE}"

# 清理旧备份（保留最近7天）
echo -e "${YELLOW}🧹 清理7天前的备份...${NC}"
find ${BACKUP_DIR} -name "cherry-etcd-backup-*.db.gz" -mtime +7 -delete

echo -e "${GREEN}🎉 备份脚本执行完成！${NC}"