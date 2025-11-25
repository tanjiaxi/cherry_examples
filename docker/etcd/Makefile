# Cherry框架 etcd Docker环境 Makefile

.PHONY: help build up down logs test backup clean status

# 默认目标
help:
	@echo "Cherry框架 etcd Docker环境"
	@echo "=========================="
	@echo "可用命令:"
	@echo "  make up      - 启动etcd服务"
	@echo "  make down    - 停止etcd服务"
	@echo "  make logs    - 查看etcd日志"
	@echo "  make test    - 测试etcd功能"
	@echo "  make backup  - 备份etcd数据"
	@echo "  make status  - 查看服务状态"
	@echo "  make clean   - 清理所有数据"
	@echo "  make build   - 构建自定义镜像"

# 启动服务
up:
	@echo "🚀 启动Cherry etcd服务..."
	@echo "127.0.0.1 dev.com" | sudo tee -a /etc/hosts || true
	docker-compose up -d
	@echo "✅ 服务启动完成"
	@echo "📡 etcd端点: http://dev.com:2379"
	@echo "🌐 Web管理界面: http://localhost:8080"

# 停止服务
down:
	@echo "🛑 停止Cherry etcd服务..."
	docker-compose down
	@echo "✅ 服务已停止"

# 查看日志
logs:
	@echo "📋 查看etcd日志..."
	docker-compose logs -f etcd

# 测试etcd功能
test:
	@echo "🧪 测试etcd功能..."
	@./scripts/test-etcd.sh

# 备份数据
backup:
	@echo "💾 备份etcd数据..."
	@./scripts/backup-etcd.sh

# 查看服务状态
status:
	@echo "📊 服务状态:"
	@docker-compose ps
	@echo ""
	@echo "🏥 健康检查:"
	@docker exec cherry-etcd etcdctl endpoint health 2>/dev/null || echo "❌ etcd服务未运行"

# 清理所有数据
clean:
	@echo "🧹 清理所有数据..."
	@read -p "确定要删除所有etcd数据吗？(y/N): " confirm && [ "$$confirm" = "y" ] || exit 1
	docker-compose down -v
	docker volume rm etcd_etcd_data 2>/dev/null || true
	@echo "✅ 清理完成"

# 构建自定义镜像
build:
	@echo "🔨 构建Cherry etcd镜像..."
	docker build -t cherry-etcd:latest .
	@echo "✅ 镜像构建完成"

# 重启服务
restart: down up

# 进入etcd容器
shell:
	@echo "🐚 进入etcd容器..."
	docker exec -it cherry-etcd sh

# 查看etcd版本
version:
	@echo "📋 etcd版本信息:"
	@docker exec cherry-etcd etcd --version 2>/dev/null || echo "❌ etcd服务未运行"

# 监控etcd性能
monitor:
	@echo "📈 etcd性能监控..."
	@docker exec cherry-etcd etcdctl endpoint status --write-out=table 2>/dev/null || echo "❌ etcd服务未运行"

# 快速启动（包含hosts配置）
quick-start: up test
	@echo "🎉 Cherry etcd环境已就绪！"