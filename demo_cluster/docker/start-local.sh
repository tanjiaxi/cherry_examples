#!/bin/bash

set -e

echo "启动 Docker 基础服务..."
docker-compose -f docker-compose-local.yml up -d

echo "等待服务就绪..."
sleep 5

echo "启动 Center 节点..."
./game-server center --path=../config/demo-cluster.json --node=gc-center-1 &
CENTER_PID=$!

echo "启动 Gate 节点..."
./game-server gate --path=../config/demo-cluster.json --node=gc-gate-1 &
GATE_PID=$!

echo "启动 Game 节点..."
./game-server game --path=../config/demo-cluster.json --node=gc-game-1 &
GAME_PID=$!

echo "启动 Web 节点..."
./game-server web --path=../config/demo-cluster.json --node=gc-web-1 &
WEB_PID=$!

echo ""
echo "所有服务已启动！"
echo "Center PID: $CENTER_PID"
echo "Gate PID: $GATE_PID"
echo "Game PID: $GAME_PID"
echo "Web PID: $WEB_PID"
echo ""
echo "Web 服务地址: http://localhost:3013"
echo ""
echo "按 Ctrl+C 停止所有服务"

# 等待所有进程
wait
