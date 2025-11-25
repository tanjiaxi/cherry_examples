#!/bin/bash

# Cherry框架etcd测试脚本
# 测试etcd是否正常工作，并模拟Cherry框架的节点注册

set -e

echo "🚀 Cherry框架 etcd 测试脚本"
echo "================================"

# 设置etcd环境变量
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=http://dev.com:2379

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试函数
test_connection() {
    echo -e "${BLUE}📡 测试etcd连接...${NC}"
    if etcdctl endpoint health; then
        echo -e "${GREEN}✅ etcd连接正常${NC}"
        return 0
    else
        echo -e "${RED}❌ etcd连接失败${NC}"
        return 1
    fi
}

test_basic_operations() {
    echo -e "${BLUE}🔧 测试基本操作...${NC}"
    
    # 写入测试数据
    echo "写入测试数据..."
    etcdctl put /test/key "test-value"
    
    # 读取测试数据
    echo "读取测试数据..."
    value=$(etcdctl get /test/key --print-value-only)
    if [ "$value" = "test-value" ]; then
        echo -e "${GREEN}✅ 基本读写操作正常${NC}"
    else
        echo -e "${RED}❌ 基本读写操作失败${NC}"
        return 1
    fi
    
    # 删除测试数据
    etcdctl del /test/key
    echo -e "${GREEN}✅ 基本操作测试完成${NC}"
}

simulate_cherry_nodes() {
    echo -e "${BLUE}🌸 模拟Cherry框架节点注册...${NC}"
    
    # 模拟Master节点注册
    echo "注册Master节点..."
    etcdctl put /cherry/nodes/gc-master-1 '{
        "nodeId": "gc-master-1",
        "nodeType": "master",
        "address": "127.0.0.1:8080",
        "settings": {}
    }'
    
    # 模拟Center节点注册
    echo "注册Center节点..."
    etcdctl put /cherry/nodes/gc-center-1 '{
        "nodeId": "gc-center-1", 
        "nodeType": "center",
        "address": "127.0.0.1:8081",
        "settings": {
            "db_id_list": {
                "center_db_id": "center_db_1"
            }
        }
    }'
    
    # 模拟Gate节点注册
    echo "注册Gate节点..."
    etcdctl put /cherry/nodes/gc-gate-1 '{
        "nodeId": "gc-gate-1",
        "nodeType": "gate", 
        "address": "127.0.0.1:10010",
        "settings": {
            "tcp_address": ":20010"
        }
    }'
    
    # 模拟Game节点注册
    echo "注册Game节点..."
    etcdctl put /cherry/nodes/gc-game-1 '{
        "nodeId": "gc-game-1",
        "nodeType": "game",
        "address": "127.0.0.1:8082", 
        "settings": {
            "db_id_list": {
                "game_db_id": "game_db_1"
            }
        }
    }'
    
    echo -e "${GREEN}✅ Cherry节点注册完成${NC}"
}

list_cherry_nodes() {
    echo -e "${BLUE}📋 列出所有Cherry节点...${NC}"
    echo "节点列表："
    etcdctl get /cherry/nodes/ --prefix --keys-only | while read key; do
        if [ -n "$key" ]; then
            nodeId=$(basename "$key")
            nodeData=$(etcdctl get "$key" --print-value-only)
            nodeType=$(echo "$nodeData" | grep -o '"nodeType":"[^"]*"' | cut -d'"' -f4)
            address=$(echo "$nodeData" | grep -o '"address":"[^"]*"' | cut -d'"' -f4)
            echo -e "  ${YELLOW}$nodeId${NC} (${nodeType}) - ${address}"
        fi
    done
}

test_watch() {
    echo -e "${BLUE}👀 测试节点变化监听...${NC}"
    echo "启动监听（5秒后自动停止）..."
    
    # 在后台启动监听
    timeout 5s etcdctl watch /cherry/nodes/ --prefix &
    watch_pid=$!
    
    # 等待1秒后添加一个测试节点
    sleep 1
    echo "添加测试节点..."
    etcdctl put /cherry/nodes/test-node '{"nodeId":"test-node","nodeType":"test","address":"127.0.0.1:9999"}'
    
    # 等待监听结束
    wait $watch_pid 2>/dev/null || true
    
    # 清理测试节点
    etcdctl del /cherry/nodes/test-node
    echo -e "${GREEN}✅ 监听测试完成${NC}"
}

test_ttl() {
    echo -e "${BLUE}⏰ 测试TTL功能...${NC}"
    
    # 创建带TTL的key（5秒过期）
    etcdctl put /cherry/ttl-test "ttl-value" --lease=$(etcdctl lease grant 5 | cut -d' ' -f2)
    echo "创建5秒TTL的key..."
    
    # 立即读取
    value=$(etcdctl get /cherry/ttl-test --print-value-only)
    if [ "$value" = "ttl-value" ]; then
        echo -e "${GREEN}✅ TTL key创建成功${NC}"
    fi
    
    echo "等待6秒后检查key是否过期..."
    sleep 6
    
    # 检查是否过期
    if etcdctl get /cherry/ttl-test --print-value-only 2>/dev/null | grep -q "ttl-value"; then
        echo -e "${RED}❌ TTL功能异常，key未过期${NC}"
    else
        echo -e "${GREEN}✅ TTL功能正常，key已过期${NC}"
    fi
}

cleanup() {
    echo -e "${BLUE}🧹 清理测试数据...${NC}"
    etcdctl del /cherry/nodes/ --prefix
    etcdctl del /test/ --prefix
    echo -e "${GREEN}✅ 清理完成${NC}"
}

show_cluster_info() {
    echo -e "${BLUE}ℹ️  集群信息...${NC}"
    echo "集群成员："
    etcdctl member list
    echo ""
    echo "端点状态："
    etcdctl endpoint status --write-out=table
    echo ""
    echo "端点健康："
    etcdctl endpoint health --write-out=table
}

# 主测试流程
main() {
    echo -e "${YELLOW}开始测试etcd环境...${NC}"
    echo ""
    
    # 检查etcdctl是否可用
    if ! command -v etcdctl &> /dev/null; then
        echo -e "${RED}❌ etcdctl命令未找到，请先安装etcd客户端${NC}"
        echo "安装方法："
        echo "  macOS: brew install etcd"
        echo "  Ubuntu: apt-get install etcd-client"
        echo "  或使用Docker: alias etcdctl='docker exec cherry-etcd etcdctl'"
        exit 1
    fi
    
    # 执行测试
    test_connection || exit 1
    echo ""
    
    test_basic_operations || exit 1
    echo ""
    
    simulate_cherry_nodes
    echo ""
    
    list_cherry_nodes
    echo ""
    
    test_watch
    echo ""
    
    test_ttl
    echo ""
    
    show_cluster_info
    echo ""
    
    # 询问是否清理
    read -p "是否清理测试数据？(y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup
    fi
    
    echo ""
    echo -e "${GREEN}🎉 所有测试完成！${NC}"
    echo -e "${BLUE}💡 提示：可以通过 http://localhost:8080 访问etcd Web管理界面${NC}"
}

# 运行主函数
main "$@"