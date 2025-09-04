#!/bin/bash

# DSL Workflow Web UI 启动脚本
# 使用方法: ./start-webui.sh

set -e

echo "🚀 启动 DSL Workflow Web UI"
echo "================================"

# 检查依赖
if ! command -v go &> /dev/null; then
    echo "❌ 错误: Go 没有安装"
    echo "请安装 Go 1.21 或更高版本: https://golang.org/dl/"
    exit 1
fi

# 切换到正确目录
cd "$(dirname "$0")"

# 构建应用
echo "📦 构建应用..."
go build -o webui main.go

if [ $? -eq 0 ]; then
    echo "✅ 构建成功"
else
    echo "❌ 构建失败"
    exit 1
fi

# 启动服务器
echo ""
echo "🌐 启动 Web 服务器..."
echo "📖 打开浏览器访问: http://localhost:8080"
echo "⏹️  按 Ctrl+C 停止服务器"
echo ""

# 启动应用
./webui