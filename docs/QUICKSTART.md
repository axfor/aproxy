# 快速入门指南

本指南将帮助你在 5 分钟内运行 MySQL-PG Proxy。

## 前置要求

- Docker 和 Docker Compose (或)
- PostgreSQL 12+ 实例
- Go 1.21+ (如果从源码构建)

## 方式 1: 使用 Docker Compose (推荐)

### 步骤 1: 创建 docker-compose.yml

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: testdb
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

  aproxy:
    image: aproxy:latest
    build:
      context: .
      dockerfile: deployments/docker/Dockerfile
    ports:
      - "3306:3306"
      - "9090:9090"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - PG_HOST=postgres
      - PG_PORT=5432
      - PG_DATABASE=testdb
      - PG_USER=postgres
      - PG_PASSWORD=postgres
    volumes:
      - ./configs/config.yaml:/app/config.yaml
```

### 步骤 2: 启动服务

```bash
docker-compose up -d
```

### 步骤 3: 测试连接

```bash
# 使用 MySQL 客户端连接
mysql -h 127.0.0.1 -P 3306 -u postgres -ppostgres

# 或使用任何 MySQL 客户端库
```

### 步骤 4: 运行测试查询

```sql
-- 创建表
CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100),
    email VARCHAR(100)
);

-- 插入数据
INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@example.com'),
    ('Bob', 'bob@example.com');

-- 查询数据
SELECT * FROM users;

-- 使用 MySQL 特有语法 (会被自动转换)
SELECT * FROM users WHERE id = ? LIMIT 10, 5;
```

## 方式 2: 本地运行

### 步骤 1: 克隆并构建

```bash
# 克隆仓库
git clone https://github.com/your-org/aproxy.git
cd aproxy

# 构建
make build
```

### 步骤 2: 配置

创建配置文件 `configs/local.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 3306

postgres:
  host: "localhost"
  port: 5432
  database: "testdb"
  user: "postgres"
  password: "postgres"
  connection_mode: "session_affinity"

observability:
  log_level: "info"
```

### 步骤 3: 启动代理

```bash
./bin/aproxy -config configs/local.yaml
```

### 步骤 4: 验证

```bash
# 健康检查
curl http://localhost:9090/health

# 查看指标
curl http://localhost:9090/metrics
```

## 方式 3: Kubernetes 快速部署

### 步骤 1: 创建命名空间

```bash
kubectl create namespace aproxy
```

### 步骤 2: 部署 PostgreSQL (测试用)

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres
  namespace: aproxy
spec:
  serviceName: postgres
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_USER
          value: postgres
        - name: POSTGRES_PASSWORD
          value: postgres
        - name: POSTGRES_DB
          value: testdb
        ports:
        - containerPort: 5432
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: aproxy
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
EOF
```

### 步骤 3: 部署代理

```bash
kubectl apply -f deployments/kubernetes/deployment.yaml
```

### 步骤 4: 获取服务地址

```bash
kubectl get svc aproxy -n aproxy
```

### 步骤 5: 连接测试

```bash
# 获取 LoadBalancer IP
PROXY_IP=$(kubectl get svc aproxy -n aproxy -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# 连接
mysql -h $PROXY_IP -P 3306 -u postgres -ppostgres
```

## 验证安装

### 1. 基本查询测试

```sql
SELECT 1;
SELECT NOW();
SELECT VERSION();
```

### 2. 数据操作测试

```sql
-- 创建表
CREATE TABLE test_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    value VARCHAR(50)
);

-- 插入数据
INSERT INTO test_table (value) VALUES ('test1'), ('test2');

-- 查询
SELECT * FROM test_table;

-- 清理
DROP TABLE test_table;
```

### 3. 预编译语句测试

```python
# Python 示例
import mysql.connector

conn = mysql.connector.connect(
    host='localhost',
    port=3306,
    user='postgres',
    password='postgres',
    database='testdb'
)

cursor = conn.cursor()

# 使用预编译语句
cursor.execute("INSERT INTO test_table (value) VALUES (%s)", ("test",))
conn.commit()

cursor.execute("SELECT * FROM test_table WHERE id = %s", (1,))
print(cursor.fetchall())

conn.close()
```

### 4. 监控检查

```bash
# 查看活跃连接
curl http://localhost:9090/metrics | grep mysql_pg_proxy_active_connections

# 查看总查询数
curl http://localhost:9090/metrics | grep mysql_pg_proxy_total_queries

# 查看错误数
curl http://localhost:9090/metrics | grep mysql_pg_proxy_errors_total
```

## 性能测试

### 使用 sysbench 测试

```bash
# 准备测试数据
sysbench /usr/share/sysbench/oltp_read_write.lua \
  --mysql-host=127.0.0.1 \
  --mysql-port=3306 \
  --mysql-user=postgres \
  --mysql-password=postgres \
  --mysql-db=testdb \
  --tables=10 \
  --table-size=10000 \
  prepare

# 运行测试
sysbench /usr/share/sysbench/oltp_read_write.lua \
  --mysql-host=127.0.0.1 \
  --mysql-port=3306 \
  --mysql-user=postgres \
  --mysql-password=postgres \
  --mysql-db=testdb \
  --tables=10 \
  --table-size=10000 \
  --threads=10 \
  --time=60 \
  --report-interval=10 \
  run

# 清理
sysbench /usr/share/sysbench/oltp_read_write.lua \
  --mysql-host=127.0.0.1 \
  --mysql-port=3306 \
  --mysql-user=postgres \
  --mysql-password=postgres \
  --mysql-db=testdb \
  --tables=10 \
  cleanup
```

## 常见问题

### Q: 无法连接到代理

**A**: 检查以下几点:
1. 代理是否正在运行: `docker ps` 或 `ps aux | grep aproxy`
2. 端口是否被占用: `netstat -tlnp | grep 3306`
3. PostgreSQL 是否可达: `psql -h localhost -U postgres`

### Q: 查询返回语法错误

**A**: 某些 MySQL 特有语法可能不支持,可以:
1. 检查 SQL 重写日志
2. 禁用 SQL 重写: `sql_rewrite.enabled: false`
3. 修改 SQL 为 PostgreSQL 兼容语法

### Q: 性能不如预期

**A**: 尝试以下优化:
1. 调整连接模式: `connection_mode: "pooled"`
2. 增加连接池大小: `max_pool_size: 200`
3. 使用预编译语句
4. 检查 PostgreSQL 性能

## 下一步

- 阅读 [设计文档](DESIGN.md) 了解架构细节
- 阅读 [运维手册](RUNBOOK.md) 了解生产部署
- 查看 [完整配置](../configs/config.yaml) 了解所有配置项

## 获取帮助

- GitHub Issues: https://github.com/your-org/aproxy/issues
- 文档: https://docs.your-org.com/aproxy
