-- 远程MySQL服务器权限设置脚本
-- 请在MySQL服务器(172.16.15.15)上执行此脚本

-- 如果使用root密码'root'连接，首先修复root用户
-- ALTER USER 'root'@'%' IDENTIFIED WITH mysql_native_password BY 'root';

-- 创建autoui用户，允许从任何主机连接
CREATE USER IF NOT EXISTS 'autoui'@'%' IDENTIFIED BY '123456';

-- 授予所有权限
GRANT ALL PRIVILEGES ON *.* TO 'autoui'@'%' WITH GRANT OPTION;

-- 创建数据库
CREATE DATABASE IF NOT EXISTS autoui_platform CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 授予数据库权限
GRANT ALL PRIVILEGES ON autoui_platform.* TO 'autoui'@'%';

-- 刷新权限
FLUSH PRIVILEGES;

-- 显示用户信息
SELECT user,host,plugin FROM mysql.user WHERE user IN ('root','autoui');