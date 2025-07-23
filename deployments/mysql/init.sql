-- Create database if not exists
CREATE DATABASE IF NOT EXISTS autoui_platform CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Use the database
USE autoui_platform;

-- Grant privileges to user
GRANT ALL PRIVILEGES ON autoui_platform.* TO 'autoui'@'%';
FLUSH PRIVILEGES;