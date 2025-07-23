# 数据库连接问题解决方案

## 🚨 当前问题
```
ERROR 1130 (HY000): Host 'RockyM' is not allowed to connect to this MariaDB server
```

这表明远程MariaDB服务器(172.16.15.15)不允许从您的主机连接。

## 🔧 解决方案

### 方案1：使用本地MySQL (推荐)

安装并使用本地MySQL服务器：

```bash
# 安装MySQL
sudo dnf install mysql-server  # 对于Rocky Linux/CentOS
# 或
sudo apt install mysql-server  # 对于Ubuntu

# 启动MySQL服务
sudo systemctl start mysqld
sudo systemctl enable mysqld

# 安全配置
sudo mysql_secure_installation

# 创建用户和数据库
sudo mysql -u root -p < scripts/mysql-setup.sql
```

然后更新.env文件：
```
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=autoui
DB_PASSWORD=123456
DB_NAME=autoui_platform
```

### 方案2：修复远程MySQL权限

需要在远程MySQL服务器(172.16.15.15)上执行：

```sql
-- 允许从您的主机连接
CREATE USER 'autoui'@'%' IDENTIFIED BY '123456';
GRANT ALL PRIVILEGES ON *.* TO 'autoui'@'%';

-- 或者允许特定主机
CREATE USER 'autoui'@'您的IP地址' IDENTIFIED BY '123456';
GRANT ALL PRIVILEGES ON *.* TO 'autoui'@'您的IP地址';

FLUSH PRIVILEGES;
```

### 方案3：使用Docker MySQL

快速启动本地MySQL容器：

```bash
# 启动MySQL容器
docker run --name autoui-mysql \
  -e MYSQL_ROOT_PASSWORD=123456 \
  -e MYSQL_DATABASE=autoui_platform \
  -e MYSQL_USER=autoui \
  -e MYSQL_PASSWORD=123456 \
  -p 3306:3306 \
  -d mysql:8.0

# 等待启动完成
sleep 30

# 测试连接
mysql -h localhost -u autoui -p123456 -e "SELECT VERSION();"
```

然后更新.env文件：
```
DB_HOST=localhost
DB_PORT=3306
DB_USERNAME=autoui
DB_PASSWORD=123456
DB_NAME=autoui_platform
```

## 🚀 快速Docker解决方案

如果您想快速解决问题，推荐使用Docker MySQL：