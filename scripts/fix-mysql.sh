#!/bin/bash

# MySQL权限修复脚本

echo "🔧 修复MySQL连接权限问题..."

# 获取MySQL版本
mysql_version=$(mysql --version | grep -oP '\d+\.\d+' | head -1)
echo "检测到MySQL版本: $mysql_version"

echo ""
echo "请执行以下步骤修复MySQL权限问题："
echo ""

echo "1️⃣ 使用sudo登录MySQL："
echo "   sudo mysql -u root"
echo ""

echo "2️⃣ 执行以下SQL命令："
echo ""
echo "-- 查看当前用户认证方式"
echo "SELECT user,authentication_string,plugin,host FROM mysql.user WHERE user='root';"
echo ""
echo "-- 修改root用户认证方式为密码认证"
echo "ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '123456';"
echo ""
echo "-- 创建新的数据库用户（推荐）"
echo "CREATE USER 'autoui'@'localhost' IDENTIFIED BY '123456';"
echo "GRANT ALL PRIVILEGES ON *.* TO 'autoui'@'localhost' WITH GRANT OPTION;"
echo ""
echo "-- 刷新权限"
echo "FLUSH PRIVILEGES;"
echo ""
echo "-- 退出MySQL"
echo "EXIT;"
echo ""

echo "3️⃣ 或者使用以下一键修复命令："
echo ""

cat << 'EOF'
sudo mysql -u root -e "
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '123456';
CREATE USER IF NOT EXISTS 'autoui'@'localhost' IDENTIFIED BY '123456';
GRANT ALL PRIVILEGES ON *.* TO 'autoui'@'localhost' WITH GRANT OPTION;
CREATE DATABASE IF NOT EXISTS autoui_platform CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
GRANT ALL PRIVILEGES ON autoui_platform.* TO 'autoui'@'localhost';
FLUSH PRIVILEGES;
"
EOF

echo ""
echo "4️⃣ 修复后，更新 .env 文件中的数据库配置："
echo ""
echo "DB_HOST=localhost"
echo "DB_PORT=3306" 
echo "DB_USERNAME=autoui"
echo "DB_PASSWORD=123456"
echo "DB_NAME=autoui_platform"
echo ""

echo "5️⃣ 测试连接："
echo "mysql -u autoui -p123456 -e 'SELECT VERSION();'"