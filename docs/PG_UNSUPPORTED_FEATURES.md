# PostgreSQL 不支持的 MySQL 特性

本文档列出了 PostgreSQL 完全不支持或无法通过 AProxy 代理实现的 MySQL 特性。

## 🚫 存储引擎相关

PostgreSQL 只有单一存储引擎，以下 MySQL 存储引擎特性无法支持：

### MyISAM 特性
- ❌ FULLTEXT 索引语法（需改用 PostgreSQL 全文搜索）
- ❌ SPATIAL 索引（需改用 PostGIS）
- ❌ 表锁定优化
- ❌ DELAY_KEY_WRITE

### InnoDB 特性
- ❌ 行级锁提示（SELECT ... FOR UPDATE SKIP LOCKED）
- ❌ InnoDB 特定的配置参数
- ❌ Tablespace 语法

## 🚫 复制和高可用

- ❌ 二进制日志（Binary Log）
- ❌ GTID（Global Transaction ID）
- ❌ 主从复制命令：
  - `CHANGE MASTER TO`
  - `START SLAVE` / `STOP SLAVE`
  - `SHOW SLAVE STATUS`
- ❌ 半同步复制
- ❌ Group Replication
- ❌ MySQL Cluster（NDB）

## 🚫 存储过程和函数

### 语法差异
- ❌ MySQL 过程语言（需重写为 PL/pgSQL）
- ❌ `DELIMITER` 语句
- ❌ `BEGIN ... END` 块（PostgreSQL 使用 `BEGIN ... END;`）
- ❌ `DECLARE` 位置不同
- ❌ `SET @变量` 语法

### 示例对比

**MySQL**:
```sql
DELIMITER $$
CREATE PROCEDURE get_user(IN user_id INT)
BEGIN
    SELECT * FROM users WHERE id = user_id;
END$$
DELIMITER ;
```

**PostgreSQL 需改写为**:
```sql
CREATE OR REPLACE FUNCTION get_user(user_id INTEGER)
RETURNS TABLE(id INTEGER, name VARCHAR) AS $$
BEGIN
    RETURN QUERY SELECT * FROM users WHERE id = user_id;
END;
$$ LANGUAGE plpgsql;
```

## 🚫 触发器

### 语法差异
- ❌ `FOR EACH ROW` 必须显式指定
- ❌ `NEW` / `OLD` 使用方式不同
- ❌ `SIGNAL` / `RESIGNAL` 语句

**MySQL**:
```sql
CREATE TRIGGER before_user_update
BEFORE UPDATE ON users
FOR EACH ROW
BEGIN
    SET NEW.updated_at = NOW();
END;
```

**PostgreSQL 需改写为**:
```sql
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER before_user_update
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();
```

## 🚫 数据类型

### 完全不支持的类型
- ❌ `ENUM`（可用 CHECK 约束或自定义类型替代）
- ❌ `SET`（可用数组或多对多表替代）
- ❌ `TINYINT(1)` 作为布尔（PostgreSQL 有真正的 BOOLEAN）
- ❌ `YEAR` 类型（可用 INTEGER 或 DATE）
- ❌ `GEOMETRY`, `POINT`, `LINESTRING`（需 PostGIS 扩展）
- ❌ 整数类型的显示宽度（如 `INT(11)`）
- ❌ `UNSIGNED` 修饰符

### 示例转换

**MySQL**:
```sql
CREATE TABLE example (
    status ENUM('active', 'inactive'),
    flags SET('read', 'write', 'execute'),
    year YEAR,
    is_admin TINYINT(1)
);
```

**PostgreSQL 替代方案**:
```sql
CREATE TYPE status_type AS ENUM ('active', 'inactive');

CREATE TABLE example (
    status status_type,
    flags TEXT[],  -- 或使用 bit string
    year INTEGER,
    is_admin BOOLEAN
);
```

## 🚫 字符集和排序规则

### 不支持的语法
- ❌ `CHARACTER SET` / `CHARSET` 子句在列定义中
- ❌ `COLLATE` 与 MySQL 不兼容
- ❌ `utf8mb4` 特指（PostgreSQL 的 UTF8 就是完整 Unicode）
- ❌ 表级字符集设置

**MySQL**:
```sql
CREATE TABLE test (
    name VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci
) CHARACTER SET=utf8mb4;
```

**PostgreSQL**:
```sql
-- PostgreSQL 在数据库级别设置编码，表/列级别不需要
CREATE TABLE test (
    name VARCHAR(100) COLLATE "en_US.UTF-8"
);
```

## 🚫 全文搜索

### MySQL 特性
- ❌ `MATCH() AGAINST()` 语法
- ❌ `FULLTEXT INDEX`
- ❌ `IN BOOLEAN MODE`
- ❌ `IN NATURAL LANGUAGE MODE`
- ❌ `WITH QUERY EXPANSION`

**PostgreSQL 需改用 `to_tsvector` 和 `to_tsquery`**:

**MySQL**:
```sql
SELECT * FROM articles
WHERE MATCH(title, content) AGAINST('search term' IN BOOLEAN MODE);
```

**PostgreSQL**:
```sql
SELECT * FROM articles
WHERE to_tsvector('english', title || ' ' || content) @@
      to_tsquery('english', 'search & term');
```

## 🚫 分区表

### 语法差异
- ❌ `PARTITION BY RANGE/LIST/HASH` MySQL 语法
- ❌ `PARTITIONS num` 子句
- ❌ `SUBPARTITION` 语法

**MySQL**:
```sql
CREATE TABLE orders (
    id INT,
    order_date DATE
) PARTITION BY RANGE (YEAR(order_date)) (
    PARTITION p2020 VALUES LESS THAN (2021),
    PARTITION p2021 VALUES LESS THAN (2022)
);
```

**PostgreSQL（10+）需改用声明式分区**:
```sql
CREATE TABLE orders (
    id INTEGER,
    order_date DATE
) PARTITION BY RANGE (order_date);

CREATE TABLE orders_2020 PARTITION OF orders
FOR VALUES FROM ('2020-01-01') TO ('2021-01-01');

CREATE TABLE orders_2021 PARTITION OF orders
FOR VALUES FROM ('2021-01-01') TO ('2022-01-01');
```

## 🚫 事件调度器

- ❌ `CREATE EVENT`
- ❌ `ALTER EVENT`
- ❌ `DROP EVENT`
- ❌ `SHOW EVENTS`

**替代方案**: 使用 `cron` 或 `pg_cron` 扩展

**MySQL**:
```sql
CREATE EVENT cleanup_old_logs
ON SCHEDULE EVERY 1 DAY
DO DELETE FROM logs WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
```

**PostgreSQL 需使用 pg_cron**:
```sql
-- 需先安装 pg_cron 扩展
SELECT cron.schedule('cleanup_old_logs', '0 0 * * *',
    $$DELETE FROM logs WHERE created_at < NOW() - INTERVAL '30 days'$$);
```

## 🚫 信息模式（Information Schema）

### 不完全兼容
- ❌ `information_schema.ENGINES`
- ❌ `information_schema.PLUGINS`
- ❌ `information_schema.PARTITIONS`
- ❌ 某些列在 PostgreSQL 中命名不同

**替代**: 使用 PostgreSQL 的 `pg_catalog` 系统表

## 🚫 用户和权限

### 不支持的语法
- ❌ `GRANT ... ON *.* ` 全局权限语法
- ❌ `IDENTIFIED BY` 在 GRANT 中创建用户
- ❌ `SHOW GRANTS FOR user@host`
- ❌ `host` 部分的用户（PostgreSQL 使用 pg_hba.conf）

**MySQL**:
```sql
GRANT ALL PRIVILEGES ON *.* TO 'admin'@'%' IDENTIFIED BY 'password';
```

**PostgreSQL**:
```sql
CREATE USER admin WITH PASSWORD 'password';
GRANT ALL PRIVILEGES ON DATABASE mydb TO admin;
-- host-based access control 在 pg_hba.conf 中配置
```

## 🚫 SQL 语法差异

### INSERT ... ON DUPLICATE KEY UPDATE
虽然 AProxy 尝试转换，但复杂情况可能失败：

**MySQL**:
```sql
INSERT INTO users (id, name, count)
VALUES (1, 'John', 1)
ON DUPLICATE KEY UPDATE count = count + VALUES(count);
```

**PostgreSQL 转换**:
```sql
INSERT INTO users (id, name, count)
VALUES (1, 'John', 1)
ON CONFLICT (id) DO UPDATE SET count = users.count + EXCLUDED.count;
```

⚠️ **限制**: `VALUES()` 函数需要改为 `EXCLUDED`

### REPLACE INTO
AProxy 转换为 `INSERT ... ON CONFLICT ... DO UPDATE`，但无法完全模拟 REPLACE 的删除后插入语义。

### LIMIT 在 UPDATE/DELETE
**MySQL**:
```sql
DELETE FROM logs WHERE created_at < '2020-01-01' LIMIT 1000;
```

**PostgreSQL 不支持 DELETE ... LIMIT**，需要子查询：
```sql
DELETE FROM logs WHERE id IN (
    SELECT id FROM logs
    WHERE created_at < '2020-01-01'
    LIMIT 1000
);
```

## 🚫 函数差异

### 完全不支持或不兼容的函数
- ❌ `GROUP_CONCAT()` 分隔符选项 `SEPARATOR '|'`（需手动调整）
- ❌ `DATE_FORMAT()` 格式字符串（需转换为 `TO_CHAR`）
- ❌ `STR_TO_DATE()`（需转换为 `TO_DATE`）
- ❌ `TIMESTAMPDIFF()`（需使用 `EXTRACT(EPOCH FROM ...)`）
- ❌ `FOUND_ROWS()`（无直接等价）
- ❌ `LAST_INSERT_ID()` 跨连接（PostgreSQL 的 RETURNING 更可靠）
- ❌ `GET_LOCK()`, `RELEASE_LOCK()`（需使用 `pg_advisory_lock`）
- ❌ `ENCRYPT()`, `DECRYPT()`（需使用 `pgcrypto` 扩展）

## 🚫 其他不支持的特性

### 系统变量
- ❌ `SET @变量 = 值`（用户变量，PostgreSQL 需使用临时表或 SESSION 变量）
- ❌ `SELECT @变量 := 值`（赋值查询）
- ❌ `@@global.variable`（全局系统变量）
- ❌ `@@session.variable`（会话变量，部分可映射）

### 其他
- ❌ `EXPLAIN EXTENDED`
- ❌ `HANDLER ... OPEN/READ/CLOSE`（低级表扫描）
- ❌ `LOAD DATA INFILE`（需使用 `COPY FROM`）
- ❌ `SELECT INTO OUTFILE`（需使用 `COPY TO`）
- ❌ `LOCK TABLES` / `UNLOCK TABLES`（PostgreSQL 锁机制不同）
- ❌ `START TRANSACTION WITH CONSISTENT SNAPSHOT`
- ❌ `XA START` / `XA COMMIT`（分布式事务，PostgreSQL 有两阶段提交但语法不同）

## ✅ 替代方案总结

| MySQL 特性 | PostgreSQL 替代方案 |
|-----------|-------------------|
| ENUM | 自定义 ENUM 类型或 CHECK 约束 |
| SET | TEXT[] 数组或 bit string |
| FULLTEXT | to_tsvector / to_tsquery |
| Event Scheduler | pg_cron 扩展 |
| 存储过程 | PL/pgSQL 函数 |
| @变量 | 临时表或自定义类型 |
| GET_LOCK() | pg_advisory_lock() |
| SPATIAL | PostGIS 扩展 |
| LOAD DATA | COPY FROM |

## 📖 参考资料

- [PostgreSQL vs MySQL 特性对比](https://wiki.postgresql.org/wiki/Things_to_find_out_about_when_moving_from_MySQL_to_PostgreSQL)
- [PostgreSQL 文档](https://www.postgresql.org/docs/)
- [MySQL 迁移到 PostgreSQL 指南](https://www.postgresql.org/docs/current/mysql-to-postgres.html)

## 💡 建议

在使用 AProxy 之前：

1. **评估依赖**: 检查应用是否使用了上述不支持的特性
2. **代码审计**: 搜索存储过程、触发器、特殊函数调用
3. **测试覆盖**: 对所有 SQL 语句进行兼容性测试
4. **逐步迁移**: 从简单查询开始，逐步迁移复杂特性
5. **性能测试**: PostgreSQL 的查询优化器与 MySQL 不同，需要重新调优

**原则**: AProxy 能处理大部分常见 SQL，但复杂的 MySQL 特定特性需要应用层改造。
