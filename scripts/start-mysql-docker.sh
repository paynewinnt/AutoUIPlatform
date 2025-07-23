#!/bin/bash

# 启动MySQL Docker容器用于AutoUI Platform

set -e

echo "🐳 启动MySQL Docker容器..."

# 检查Docker是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker未运行，请先启动Docker服务"
    exit 1
fi

# 停止并删除已存在的容器
if docker ps -a | grep -q autoui-mysql; then
    echo "🛑 停止已存在的MySQL容器..."
    docker stop autoui-mysql || true
    docker rm autoui-mysql || true
fi

# 启动新的MySQL容器
echo "🚀 启动新的MySQL容器..."
docker run --name autoui-mysql \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_DATABASE=autoui_platform \
  -e MYSQL_USER=autoui \
  -e MYSQL_PASSWORD=123456 \
  -p 3306:3306 \
  -d mysql:8.0

echo "⏳ 等待MySQL启动完成..."
sleep 20

# 测试连接
echo "🔍 测试数据库连接..."
max_attempts=30
attempt=1

while [ $attempt -le $max_attempts ]; do
    if mysql -h localhost -P 3306 -u autoui -p123456 -e "SELECT 1;" > /dev/null 2>&1; then
        echo "✅ MySQL连接成功！"
        break
    else
        echo "等待MySQL启动... ($attempt/$max_attempts)"
        sleep 2
        attempt=$((attempt + 1))
    fi
done

if [ $attempt -gt $max_attempts ]; then
    echo "❌ MySQL启动超时，请检查Docker日志："
    echo "docker logs autoui-mysql"
    exit 1
fi

# 显示连接信息
echo ""
echo "🎉 MySQL Docker容器启动成功！"
echo ""
echo "📊 连接信息："
echo "   主机: localhost"
echo "   端口: 3306"
echo "   用户: autoui"
echo "   密码: 123456"
echo "   数据库: autoui_platform"
echo ""
echo "🔧 请确保.env文件配置如下："
echo "   DB_HOST=localhost"
echo "   DB_PORT=3306"
echo "   DB_USERNAME=autoui"
echo "   DB_PASSWORD=123456"
echo "   DB_NAME=autoui_platform"
echo ""
echo "📋 有用命令："
echo "   查看容器状态: docker ps"
echo "   查看容器日志: docker logs autoui-mysql"
echo "   连接数据库: mysql -h localhost -u autoui -p123456"
echo "   停止容器: docker stop autoui-mysql"
echo "   删除容器: docker rm autoui-mysql"