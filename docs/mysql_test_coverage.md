# MySQL 5.7 官方测试套件分析与覆盖计划

基于对MySQL 5.7官方测试套件（5129个测试文件）的深入分析，本文档记录AProxy需要覆盖的SQL语法特性。

## 测试套件概览

| 类别 | 文件数 | 优先级 | 说明 |
|------|--------|--------|------|
| DDL (CREATE/ALTER/DROP) | 51 | P0 | 数据定义语言 |
| DML (SELECT/INSERT/UPDATE/DELETE) | 45 | P0 | 数据操作语言 |
| JOIN操作 | 16 | P0 | 表连接 |
| 函数 | 59 | P1 | MySQL内置函数 |
| 子查询和UNION | 49 | P1 | 复杂查询 |
| 数据类型 | 126 | P0 | 各种数据类型 |
| 索引和约束 | 26 | P0 | INDEX, UNIQUE, FOREIGN KEY |

## 核心SQL语法覆盖清单

### 1. DDL语句 (Data Definition Language)

#### 1.1 CREATE TABLE

**基础语法:**
```sql
-- 简单CREATE TABLE
CREATE TABLE t1 (id INT PRIMARY KEY, name VARCHAR(100));

-- 带AUTO_INCREMENT
CREATE TABLE t1 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    value VARCHAR(100)
);

-- IF NOT EXISTS
CREATE TABLE IF NOT EXISTS t1 (id INT);

-- 临时表
CREATE TEMPORARY TABLE t1 (id INT);
```

**列定义:**
```sql
-- 数据类型完整覆盖
CREATE TABLE types_test (
    -- 整数类型
    tiny TINYINT,
    tiny_unsigned TINYINT UNSIGNED,
    small SMALLINT,
    medium MEDIUMINT,
    normal INT,
    big BIGINT,

    -- 浮点类型
    f FLOAT,
    d DOUBLE,
    dec DECIMAL(10,2),

    -- 字符串类型
    c CHAR(10),
    vc VARCHAR(100),
    tiny_text TINYTEXT,
    txt TEXT,
    medium_text MEDIUMTEXT,
    long_text LONGTEXT,

    -- 二进制类型
    bin BINARY(10),
    vbin VARBINARY(100),
    tiny_blob TINYBLOB,
    bl BLOB,
    medium_blob MEDIUMBLOB,
    long_blob LONGBLOB,

    -- 日期时间类型
    d DATE,
    dt DATETIME,
    ts TIMESTAMP,
    t TIME,
    y YEAR,

    -- 特殊类型
    e ENUM('a','b','c'),
    s SET('x','y','z'),
    j JSON,

    -- 空间类型 (GIS)
    geo GEOMETRY,
    pt POINT,
    ls LINESTRING,
    poly POLYGON
);
```

**约束和索引:**
```sql
-- PRIMARY KEY
CREATE TABLE t1 (id INT PRIMARY KEY);
CREATE TABLE t1 (id INT, PRIMARY KEY(id));
CREATE TABLE t1 (a INT, b INT, PRIMARY KEY(a,b)); -- 复合主键

-- UNIQUE
CREATE TABLE t1 (email VARCHAR(100) UNIQUE);
CREATE TABLE t1 (id INT, UNIQUE KEY idx_id (id));

-- INDEX/KEY
CREATE TABLE t1 (
    id INT,
    name VARCHAR(100),
    email VARCHAR(100),
    INDEX idx_name (name),
    KEY idx_email (email)
);

-- 复合索引
CREATE TABLE t1 (
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    INDEX idx_name (first_name, last_name)
);

-- FULLTEXT索引 (MyISAM/InnoDB)
CREATE TABLE t1 (
    title VARCHAR(200),
    content TEXT,
    FULLTEXT idx_content (title, content)
);

-- FOREIGN KEY
CREATE TABLE parent (id INT PRIMARY KEY);
CREATE TABLE child (
    id INT PRIMARY KEY,
    parent_id INT,
    FOREIGN KEY (parent_id) REFERENCES parent(id)
        ON DELETE CASCADE
        ON UPDATE RESTRICT
);

-- CHECK约束 (MySQL 8.0+)
CREATE TABLE t1 (
    age INT CHECK (age >= 0 AND age <= 150)
);
```

**DEFAULT值:**
```sql
CREATE TABLE t1 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    status VARCHAR(20) DEFAULT 'pending',
    count INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
```

**表选项:**
```sql
CREATE TABLE t1 (
    id INT
) ENGINE=InnoDB
  DEFAULT CHARSET=utf8mb4
  COLLATE=utf8mb4_unicode_ci
  AUTO_INCREMENT=1000
  COMMENT='用户表';
```

#### 1.2 ALTER TABLE

```sql
-- 添加列
ALTER TABLE t1 ADD COLUMN age INT;
ALTER TABLE t1 ADD COLUMN email VARCHAR(100) AFTER name;
ALTER TABLE t1 ADD COLUMN status VARCHAR(20) FIRST;

-- 修改列
ALTER TABLE t1 MODIFY COLUMN name VARCHAR(200);
ALTER TABLE t1 CHANGE COLUMN old_name new_name VARCHAR(100);

-- 删除列
ALTER TABLE t1 DROP COLUMN age;

-- 添加索引
ALTER TABLE t1 ADD INDEX idx_name (name);
ALTER TABLE t1 ADD UNIQUE KEY idx_email (email);
ALTER TABLE t1 ADD PRIMARY KEY (id);

-- 删除索引
ALTER TABLE t1 DROP INDEX idx_name;
ALTER TABLE t1 DROP PRIMARY KEY;

-- 重命名表
ALTER TABLE t1 RENAME TO t2;
RENAME TABLE t1 TO t2;

-- 修改表选项
ALTER TABLE t1 ENGINE=InnoDB;
ALTER TABLE t1 AUTO_INCREMENT=1000;
```

#### 1.3 DROP TABLE

```sql
DROP TABLE t1;
DROP TABLE IF EXISTS t1;
DROP TABLE t1, t2, t3;  -- 删除多个表
DROP TEMPORARY TABLE t1;
```

#### 1.4 CREATE/DROP INDEX

```sql
CREATE INDEX idx_name ON t1 (name);
CREATE UNIQUE INDEX idx_email ON t1 (email);
CREATE INDEX idx_composite ON t1 (col1, col2, col3);

DROP INDEX idx_name ON t1;
```

### 2. DML语句 (Data Manipulation Language)

#### 2.1 INSERT

```sql
-- 基础INSERT
INSERT INTO t1 (name, age) VALUES ('Alice', 30);

-- INSERT多行
INSERT INTO t1 (name, age) VALUES
    ('Alice', 30),
    ('Bob', 25),
    ('Charlie', 35);

-- INSERT ... SELECT
INSERT INTO t1 (name, age)
SELECT name, age FROM t2 WHERE age > 18;

-- INSERT IGNORE (忽略重复键错误)
INSERT IGNORE INTO t1 (id, name) VALUES (1, 'Alice');

-- REPLACE INTO (存在则替换)
REPLACE INTO t1 (id, name) VALUES (1, 'Alice');

-- ON DUPLICATE KEY UPDATE
INSERT INTO t1 (id, name, count) VALUES (1, 'Alice', 1)
ON DUPLICATE KEY UPDATE count = count + 1;

-- INSERT with DEFAULT
INSERT INTO t1 (name) VALUES (DEFAULT);

-- INSERT with expressions
INSERT INTO t1 (name, created_at) VALUES ('Alice', NOW());
```

#### 2.2 SELECT

**基础查询:**
```sql
SELECT * FROM t1;
SELECT id, name FROM t1;
SELECT DISTINCT name FROM t1;
SELECT name AS user_name FROM t1;
```

**WHERE子句:**
```sql
SELECT * FROM t1 WHERE age > 25;
SELECT * FROM t1 WHERE name = 'Alice';
SELECT * FROM t1 WHERE age BETWEEN 20 AND 30;
SELECT * FROM t1 WHERE name IN ('Alice', 'Bob');
SELECT * FROM t1 WHERE name LIKE 'A%';
SELECT * FROM t1 WHERE name IS NULL;
SELECT * FROM t1 WHERE name IS NOT NULL;

-- 复合条件
SELECT * FROM t1 WHERE age > 25 AND status = 'active';
SELECT * FROM t1 WHERE age > 30 OR salary > 50000;
SELECT * FROM t1 WHERE NOT (age < 18);
```

**ORDER BY:**
```sql
SELECT * FROM t1 ORDER BY age;
SELECT * FROM t1 ORDER BY age DESC;
SELECT * FROM t1 ORDER BY age ASC, name DESC;
SELECT * FROM t1 ORDER BY RAND();  -- 随机排序
```

**LIMIT和OFFSET:**
```sql
SELECT * FROM t1 LIMIT 10;
SELECT * FROM t1 LIMIT 10 OFFSET 20;
SELECT * FROM t1 LIMIT 20, 10;  -- MySQL风格：offset, count
```

**GROUP BY和聚合函数:**
```sql
SELECT COUNT(*) FROM t1;
SELECT COUNT(DISTINCT name) FROM t1;
SELECT department, COUNT(*) FROM t1 GROUP BY department;
SELECT department, AVG(salary) FROM t1 GROUP BY department;
SELECT department, MIN(age), MAX(age), SUM(salary)
FROM t1 GROUP BY department;

-- HAVING
SELECT department, AVG(salary) as avg_sal
FROM t1
GROUP BY department
HAVING avg_sal > 50000;
```

**JOIN操作:**
```sql
-- INNER JOIN
SELECT * FROM t1 INNER JOIN t2 ON t1.id = t2.user_id;
SELECT * FROM t1 JOIN t2 USING (id);

-- LEFT JOIN
SELECT * FROM t1 LEFT JOIN t2 ON t1.id = t2.user_id;

-- RIGHT JOIN
SELECT * FROM t1 RIGHT JOIN t2 ON t1.id = t2.user_id;

-- CROSS JOIN
SELECT * FROM t1 CROSS JOIN t2;

-- 多表JOIN
SELECT *
FROM t1
JOIN t2 ON t1.id = t2.user_id
JOIN t3 ON t2.id = t3.order_id;

-- SELF JOIN
SELECT a.name, b.name
FROM employees a
JOIN employees b ON a.manager_id = b.id;
```

**子查询:**
```sql
-- WHERE子句中的子查询
SELECT * FROM t1 WHERE id IN (SELECT user_id FROM t2);
SELECT * FROM t1 WHERE age > (SELECT AVG(age) FROM t1);

-- FROM子句中的子查询（派生表）
SELECT * FROM (
    SELECT * FROM t1 WHERE age > 25
) AS subquery;

-- EXISTS
SELECT * FROM t1 WHERE EXISTS (
    SELECT 1 FROM t2 WHERE t2.user_id = t1.id
);

-- 标量子查询
SELECT name, (SELECT COUNT(*) FROM orders WHERE user_id = t1.id) as order_count
FROM t1;
```

**UNION:**
```sql
SELECT name FROM t1
UNION
SELECT name FROM t2;

-- UNION ALL (保留重复)
SELECT name FROM t1
UNION ALL
SELECT name FROM t2;
```

#### 2.3 UPDATE

```sql
-- 基础UPDATE
UPDATE t1 SET name = 'Alice' WHERE id = 1;

-- UPDATE多列
UPDATE t1 SET name = 'Alice', age = 31 WHERE id = 1;

-- UPDATE with expressions
UPDATE t1 SET count = count + 1 WHERE id = 1;
UPDATE t1 SET updated_at = NOW() WHERE id = 1;

-- UPDATE with JOIN
UPDATE t1
JOIN t2 ON t1.id = t2.user_id
SET t1.status = 'active'
WHERE t2.verified = 1;

-- UPDATE all rows
UPDATE t1 SET status = 'pending';

-- UPDATE with LIMIT
UPDATE t1 SET status = 'archived' WHERE age > 60 LIMIT 10;

-- UPDATE with ORDER BY
UPDATE t1 SET priority = priority + 1 ORDER BY created_at LIMIT 5;
```

#### 2.4 DELETE

```sql
-- 基础DELETE
DELETE FROM t1 WHERE id = 1;

-- DELETE多行
DELETE FROM t1 WHERE age < 18;

-- DELETE all rows
DELETE FROM t1;
TRUNCATE TABLE t1;  -- 更快，重置AUTO_INCREMENT

-- DELETE with LIMIT
DELETE FROM t1 WHERE status = 'inactive' LIMIT 100;

-- DELETE with JOIN
DELETE t1 FROM t1
JOIN t2 ON t1.id = t2.user_id
WHERE t2.deleted = 1;
```

### 3. MySQL函数

#### 3.1 字符串函数

```sql
-- 字符串操作
CONCAT('Hello', ' ', 'World')
CONCAT_WS(',', 'a', 'b', 'c')  -- 使用分隔符
SUBSTRING('Hello', 1, 3)
LEFT('Hello', 2)
RIGHT('Hello', 2)
LENGTH('Hello')
CHAR_LENGTH('Hello')  -- 字符数
LOWER('HELLO')
UPPER('hello')
TRIM('  hello  ')
LTRIM('  hello')
RTRIM('hello  ')
REPLACE('Hello World', 'World', 'MySQL')
REVERSE('Hello')
REPEAT('abc', 3)
LPAD('5', 3, '0')  -- '005'
RPAD('5', 3, '0')  -- '500'

-- 正则表达式
SELECT * FROM t1 WHERE name REGEXP '^A';
SELECT * FROM t1 WHERE name RLIKE '[0-9]';
```

#### 3.2 数值函数

```sql
ABS(-5)
CEIL(4.3)  -- 5
FLOOR(4.7)  -- 4
ROUND(4.567, 2)  -- 4.57
TRUNCATE(4.567, 2)  -- 4.56
MOD(10, 3)  -- 1
POW(2, 3)  -- 8
SQRT(16)  -- 4
RAND()  -- 0到1之间的随机数
GREATEST(1, 2, 3)  -- 3
LEAST(1, 2, 3)  -- 1
```

#### 3.3 日期时间函数

```sql
NOW()
CURDATE()
CURTIME()
CURRENT_TIMESTAMP()
DATE('2025-01-15 10:30:00')  -- '2025-01-15'
TIME('2025-01-15 10:30:00')  -- '10:30:00'
YEAR('2025-01-15')  -- 2025
MONTH('2025-01-15')  -- 1
DAY('2025-01-15')  -- 15
HOUR('10:30:00')  -- 10
MINUTE('10:30:00')  -- 30
SECOND('10:30:00')  -- 0
UNIX_TIMESTAMP()
FROM_UNIXTIME(1234567890)
DATE_ADD('2025-01-15', INTERVAL 1 DAY)
DATE_SUB('2025-01-15', INTERVAL 1 MONTH)
DATEDIFF('2025-01-15', '2025-01-01')  -- 14
DATE_FORMAT('2025-01-15', '%Y-%m-%d')
STR_TO_DATE('15-01-2025', '%d-%m-%Y')
```

#### 3.4 聚合函数

```sql
COUNT(*)
COUNT(DISTINCT column)
SUM(column)
AVG(column)
MIN(column)
MAX(column)
GROUP_CONCAT(column)  -- MySQL特有
GROUP_CONCAT(column SEPARATOR ',')
STD(column)  -- 标准差
VARIANCE(column)  -- 方差
```

#### 3.5 控制流函数

```sql
IF(condition, true_value, false_value)
IFNULL(expr1, expr2)
NULLIF(expr1, expr2)
COALESCE(val1, val2, val3, ...)  -- 返回第一个非NULL值

CASE
    WHEN age < 18 THEN 'minor'
    WHEN age < 65 THEN 'adult'
    ELSE 'senior'
END
```

#### 3.6 类型转换函数

```sql
CAST('123' AS SIGNED)
CAST('2025-01-15' AS DATE)
CONVERT('123', SIGNED)
BINARY 'abc'  -- 转为二进制字符串
```

### 4. 高级特性

#### 4.1 事务

```sql
START TRANSACTION;
BEGIN;

INSERT INTO t1 VALUES (1, 'Alice');
UPDATE t2 SET count = count + 1;

COMMIT;
ROLLBACK;

-- 保存点
SAVEPOINT sp1;
ROLLBACK TO SAVEPOINT sp1;
RELEASE SAVEPOINT sp1;

-- 隔离级别
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
```

#### 4.2 视图

```sql
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE age > 18;
CREATE OR REPLACE VIEW v1 AS SELECT * FROM t1;
DROP VIEW v1;
```

#### 4.3 准备语句 (Prepared Statements)

```sql
PREPARE stmt FROM 'SELECT * FROM t1 WHERE id = ?';
SET @id = 1;
EXECUTE stmt USING @id;
DEALLOCATE PREPARE stmt;
```

#### 4.4 存储过程和函数

```sql
-- 存储过程
DELIMITER //
CREATE PROCEDURE get_user(IN user_id INT)
BEGIN
    SELECT * FROM users WHERE id = user_id;
END //
DELIMITER ;

CALL get_user(1);
DROP PROCEDURE get_user;

-- 存储函数
DELIMITER //
CREATE FUNCTION get_age(birth_date DATE) RETURNS INT
DETERMINISTIC
BEGIN
    RETURN YEAR(CURDATE()) - YEAR(birth_date);
END //
DELIMITER ;

SELECT get_age('1990-01-01');
DROP FUNCTION get_age;
```

#### 4.5 触发器

```sql
CREATE TRIGGER before_insert_user
BEFORE INSERT ON users
FOR EACH ROW
BEGIN
    SET NEW.created_at = NOW();
END;

DROP TRIGGER before_insert_user;
```

### 5. 系统和元数据查询

```sql
-- 数据库信息
SHOW DATABASES;
SHOW TABLES;
SHOW COLUMNS FROM t1;
SHOW INDEX FROM t1;
SHOW CREATE TABLE t1;
DESCRIBE t1;
DESC t1;

-- 服务器状态
SHOW STATUS;
SHOW VARIABLES;
SHOW PROCESSLIST;

-- 权限
SHOW GRANTS FOR 'user'@'host';

-- INFORMATION_SCHEMA
SELECT * FROM INFORMATION_SCHEMA.TABLES;
SELECT * FROM INFORMATION_SCHEMA.COLUMNS;
```

## AProxy当前覆盖状态

### ✅ 已实现

1. **DDL**
   - CREATE TABLE (基础、AUTO_INCREMENT、INDEX、UNIQUE、多列索引)
   - 数据类型转换 (INT, VARCHAR, DATETIME, ENUM等)
   - INDEX提取和转换

2. **DML**
   - SELECT (基础、WHERE、LIMIT转换)
   - INSERT (单行、多行)
   - UPDATE (基础)
   - DELETE (基础)

3. **函数**
   - NOW() → CURRENT_TIMESTAMP
   - UNIX_TIMESTAMP() → EXTRACT(EPOCH FROM ...)
   - IFNULL() → COALESCE()

### ⚠️ 部分实现/需改进

1. **Prepared Statements** - 占位符转换有问题
2. **NULL AUTO_INCREMENT** - PostgreSQL SERIAL不接受NULL
3. **JOIN** - 未完全测试
4. **子查询** - 未完全测试
5. **聚合函数** - 部分支持

### ❌ 未实现

1. **DDL**
   - ALTER TABLE
   - DROP TABLE
   - CREATE/DROP INDEX (独立语句)
   - FOREIGN KEY
   - CHECK约束

2. **DML**
   - REPLACE INTO
   - ON DUPLICATE KEY UPDATE
   - INSERT ... SELECT
   - 多表UPDATE/DELETE

3. **高级特性**
   - 存储过程
   - 存储函数
   - 触发器
   - 视图
   - UNION

4. **大量MySQL函数**
   - 字符串函数 (90%)
   - 日期时间函数 (85%)
   - 数学函数 (100%)
   - JSON函数 (100%)

## 优先级建议

### P0 (必须)
1. 修复Prepared Statement占位符转换
2. 修复NULL AUTO_INCREMENT处理
3. 完整的JOIN支持
4. ALTER TABLE基础操作
5. 常用MySQL函数转换

### P1 (重要)
1. 子查询支持
2. UNION支持
3. REPLACE INTO
4. ON DUPLICATE KEY UPDATE
5. 更多聚合函数

### P2 (可选)
1. 存储过程/函数
2. 触发器
3. 视图
4. 全文索引
5. GIS功能

## 测试策略

1. **单元测试** - 每个SQL语法特性都有专门的测试
2. **集成测试** - 真实场景的端到端测试
3. **兼容性测试** - 基于MySQL官方测试用例
4. **性能测试** - 确保SQL重写开销可接受
5. **回归测试** - 防止新特性破坏已有功能
