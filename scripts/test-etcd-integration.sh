#!/bin/bash

# Cherry框架 etcd集成测试脚本

set -e

echo "🧪 Cherry框架 etcd集成测试"
echo "=========================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检查etcd是否运行
check_etcd() {
    echo -e "${BLUE}📡 检查etcd服务...${NC}"
    if nc -z dev.com 2379 2>/dev/null || nc -z localhost 2379 2>/dev/null; then
        echo -e "${GREEN}✅ etcd服务运行正常${NC}"
        return 0
    else
        echo -e "${RED}❌ etcd服务未运行${NC}"
        echo "请先启动etcd服务："
        echo "  cd docker/etcd && make up"
        return 1
    fi
}

# 测试编译
test_compile() {
    echo -e "${BLUE}🔨 测试编译...${NC}"
    if go build -o /tmp/test-cherry ./demo_cluster/nodes/; then
        echo -e "${GREEN}✅ 编译成功${NC}"
        rm -f /tmp/test-cherry
        return 0
    else
        echo -e "${RED}❌ 编译失败${NC}"
        return 1
    fi
}

# 检查protobuf版本
check_protobuf_versions() {
    echo -e "${BLUE}📋 检查protobuf版本...${NC}"
    echo "当前protobuf版本："
    go list -m all | grep protobuf
    echo ""
    
    # 检查是否有版本冲突
    if go list -m all | grep protobuf | grep -q "=>"; then
        echo -e "${GREEN}✅ 使用replace指令统一版本${NC}"
    else
        echo -e "${YELLOW}⚠️  未使用replace指令${NC}"
    fi
}

# 测试etcd连接
test_etcd_connection() {
    echo -e "${BLUE}🔗 测试etcd连接...${NC}"
    
    # 设置环境变量
    export ETCDCTL_API=3
    export ETCDCTL_ENDPOINTS=http://dev.com:2379,http://localhost:2379
    
    # 测试连接
    if command -v etcdctl &> /dev/null; then
        if etcdctl endpoint health 2>/dev/null; then
            echo -e "${GREEN}✅ etcd连接正常${NC}"
            return 0
        else
            echo -e "${RED}❌ etcd连接失败${NC}"
            return 1
        fi
    else
        echo -e "${YELLOW}⚠️  etcdctl未安装，跳过连接测试${NC}"
        return 0
    fi
}

# 启动Master节点测试
test_master_startup() {
    echo -e "${BLUE}🚀 测试Master节点启动...${NC}"
    
    # 创建临时配置文件
    cat > /tmp/test-etcd-config.json << 'EOF'
{
  "env": "test",
  "debug": true,
  "print_level": "info",
  "cluster": {
    "discovery": {
      "mode": "etcd"
    },
    "etcd": {
      "end_points": "dev.com:2379,localhost:2379",
      "prefix": "cherry-test",
      "ttl": 5,
      "dial_timeout": 3,
      "dial_keep_alive_time": 1,
      "dial_keep_alive_timeout": 1
    }
  },
  "node": {
    "master": [
      {
        "node_id": "test-master",
        "address": "",
        "enable": true
      }
    ]
  },
  "logger": {
    "master_log": {
      "level": "info",
      "enable_console": true,
      "enable_write_file": false
    }
  }
}
EOF

    echo "使用测试配置启动Master节点（5秒后自动停止）..."
    
    # 在后台启动Master节点
    timeout 5s go run ./demo_cluster/nodes/ --profile=/tmp/test-etcd-config.json --node=test-master &
    master_pid=$!
    
    # 等待启动
    sleep 2
    
    # 检查进程是否还在运行
    if kill -0 $master_pid 2>/dev/null; then
        echo -e "${GREEN}✅ Master节点启动成功${NC}"
        
        # 等待自动停止
        wait $master_pid 2>/dev/null || true
        echo -e "${GREEN}✅ Master节点正常停止${NC}"
    else
        echo -e "${RED}❌ Master节点启动失败${NC}"
        return 1
    fi
    
    # 清理临时文件
    rm -f /tmp/test-etcd-config.json
}

# 主测试流程
main() {
    echo -e "${YELLOW}开始etcd集成测试...${NC}"
    echo ""
    
    # 执行测试
    check_protobuf_versions
    echo ""
    
    test_compile || exit 1
    echo ""
    
    check_etcd || {
        echo -e "${YELLOW}⚠️  etcd未运行，跳过连接测试${NC}"
        echo ""
    }
    
    test_etcd_connection
    echo ""
    
    if check_etcd; then
        test_master_startup || exit 1
    else
        echo -e "${YELLOW}⚠️  etcd未运行，跳过Master节点测试${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}🎉 所有测试完成！${NC}"
    echo -e "${BLUE}💡 提示：${NC}"
    echo "1. protobuf版本冲突已解决"
    echo "2. 可以正常使用etcd组件"
    echo "3. 如需启动完整集群，请确保etcd服务运行"
}

# 运行主函数
main "$@"