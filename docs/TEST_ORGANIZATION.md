# 测试用例组织策略

## 测试分类原则

根据 PostgreSQL 兼容性,我们将测试分为两类:

1. **主测试集** (`test/integration/`): PostgreSQL 有对应实现的功能
2. **PG 不支持测试集** (`test/pg-unsupported/`): PostgreSQL 完全不支持的 MySQL 特性

## 当前测试状态

### 📊 测试通过率: 58/65 (89%)

#### 基础功能测试 (`basic_test.go`) - 8/8 ✅

| 测试名称 | PG 支持 | 位置 | 状态 |
|---------|---------|------|------|
| TestBasicQuery | ✅ | 主测试集 | PASS |
| TestCreateTable | ✅ | 主测试集 | PASS |
| TestInsertAndSelect | ✅ | 主测试集 | PASS |
| TestPreparedStatements | ✅ | 主测试集 | PASS |
| TestTransaction | ✅ | 主测试集 | PASS |
| TestShowCommands | ✅ | 主测试集 | PASS |
| TestUpdateAndDelete | ✅ | 主测试集 | PASS |
| TestConcurrentConnections | ✅ | 主测试集 | PASS |

#### MySQL 兼容性测试 (`mysql_compat_test.go`) - 13/16 🟡

| 测试名称 | PG 支持 | 位置 | 状态 | 备注 |
|---------|---------|------|------|------|
| TestDataTypes_Integer | ✅ | 主测试集 | PASS | |
| TestDataTypes_FloatingPoint | ✅ | 主测试集 | PASS | |
| TestDataTypes_String | ✅ | 主测试集 | PASS | |
| TestDataTypes_DateTime | ✅ | 主测试集 | FAIL | 待修复 |
| TestFunctions_DateTime | ✅ | 主测试集 | PASS | |
| TestFunctions_String | ✅ | 主测试集 | FAIL | 待修复 |
| TestFunctions_Aggregate | ✅ | 主测试集 | PASS | |
| TestComplexQueries_Joins | ✅ | 主测试集 | PASS | |
| TestComplexQueries_Subqueries | ✅ | 主测试集 | PASS | |
| TestComplexQueries_GroupBy | ✅ | 主测试集 | PASS | ⭐ 2025-11-07 修复 |
| TestLimitOffset | ✅ | 主测试集 | PASS | |
| TestNullValues | ✅ | 主测试集 | PASS | ⭐ 2025-11-07 修复 |
| TestBatchOperations | ✅ | 主测试集 | PASS | ⭐ 2025-11-07 修复 |
| TestIndexes | ✅ | 主测试集 | PASS | |
| TestMySQLCompatibility_INSERT | ✅ | 主测试集 | FAIL | 待修复 |
| TestMySQLCompatibility_Transactions | ✅ | 主测试集 | PASS | |

#### 学生管理测试 (`student_test.go`) - 2/6 🔴

| 测试名称 | 状态 | 备注 |
|---------|------|------|
| TestStudentTable | PASS | |
| TestStudentAutocommit | PASS | |
| TestStudentTransactions | FAIL | 待修复 |
| TestStudentSQLRewrite | FAIL | 待修复 |
| TestStudentConcurrentTransactions | FAIL | 待修复 |
| TestStudentComplexScenarios | FAIL | 待修复 |

#### MySQL 兼容性 DDL/DML 测试 - 6/6 ✅

| 测试名称 | 状态 |
|---------|------|
| TestMySQLCompatibility_DDL | PASS |
| TestMySQLCompatibility_SELECT | PASS |
| TestMySQLCompatibility_UPDATE | PASS |
| TestMySQLCompatibility_DELETE | PASS |
| TestMySQLCompatibility_DataTypes | PASS |
| TestMySQLCompatibility_Functions | PASS |

## 未来需要隔离的测试类型

当添加以下功能的测试时,应放入 `test/integration/pg-unsupported/`:

### 🚫 MySQL 特有数据类型

详见 [test/pg-unsupported/mysql_specific_types_test.go](../test/pg-unsupported/mysql_specific_types_test.go):
- `TestMySQLSpecific_ENUM` - ENUM 类型
- `TestMySQLSpecific_SET` - SET 类型
- `TestMySQLSpecific_YEAR` - YEAR 类型
- `TestMySQLSpecific_UNSIGNED` - UNSIGNED 修饰符
- `TestMySQLSpecific_MEDIUMINT` - MEDIUMINT 类型
- `TestMySQLSpecific_SpatialTypes` - GEOMETRY, POINT 等空间类型

### 🚫 MySQL 特有语法

详见 [test/pg-unsupported/mysql_specific_syntax_test.go](../test/pg-unsupported/mysql_specific_syntax_test.go):
- `TestMySQLSpecific_REPLACE_INTO` - REPLACE INTO 语句
- `TestMySQLSpecific_INSERT_VALUES_Function` - VALUES() 函数在 UPDATE 中
- `TestMySQLSpecific_UPDATE_LIMIT` - UPDATE ... LIMIT
- `TestMySQLSpecific_DELETE_LIMIT` - DELETE ... LIMIT
- `TestMySQLSpecific_FORCE_INDEX` - FORCE INDEX 提示
- `TestMySQLSpecific_PARTITION_Syntax` - MySQL 分区语法

### 🚫 MySQL 特有函数

详见 [test/pg-unsupported/mysql_specific_functions_test.go](../test/pg-unsupported/mysql_specific_functions_test.go):
- `TestMySQLSpecific_MATCH_AGAINST` - MATCH() AGAINST() 全文搜索
- `TestMySQLSpecific_FOUND_ROWS` - FOUND_ROWS() 函数
- `TestMySQLSpecific_GET_LOCK` - GET_LOCK() 命名锁
- `TestMySQLSpecific_DATE_FORMAT` - DATE_FORMAT() 日期格式化
- `TestMySQLSpecific_TIMESTAMPDIFF` - TIMESTAMPDIFF() 时间差
- `TestMySQLSpecific_INET_ATON` - IP 地址转换

## 测试运行策略

### 运行所有支持的测试

```bash
make test
# 或
make test-integration
```

### 运行集成测试

```bash
make test-integration
# 仅运行 integration tests (test/integration/)
```

### 运行 PG 不支持的测试

```bash
make test-pg-unsupported
# 运行 test/pg-unsupported/ 中的测试
# 注意: 大部分测试会被 t.Skip() 跳过
```

### 运行特定测试

```bash
# 运行基础测试
INTEGRATION_TEST=1 go test -v -run TestBasicQuery ./test/integration/

# 运行 MySQL 兼容性测试
INTEGRATION_TEST=1 go test -v -run TestDataTypes ./test/integration/

# 运行特定的不支持特性测试
INTEGRATION_TEST=1 go test -v -run TestMySQLSpecific_ENUM ./test/pg-unsupported/
```

## 添加新测试的决策流程

```
新测试用例
    │
    ├─ PostgreSQL 有对应实现?
    │   ├─ 是 → test/integration/
    │   │        (例: LIMIT OFFSET, GROUP_CONCAT → string_agg)
    │   │
    │   └─ 否 → PostgreSQL 完全不支持?
    │            ├─ 是 → test/integration/pg-unsupported/
    │            │        (例: ENUM, SET, REPLACE INTO)
    │            │
    │            └─ 否 → 需要应用层改造
    │                     → 添加到对应测试集并在文档中说明限制
```

## 测试覆盖率目标

- **主测试集**: 覆盖所有 AProxy 应该支持的功能
- **PG 不支持测试集**: 验证错误处理和优雅降级

### 当前覆盖情况

```
pkg/mapper:          37.1%
pkg/sqlrewrite:      59.7%
Integration Tests:   58/65 PASS (89%)
  - 基础功能测试:    8/8   PASS (100%)
  - MySQL 兼容测试: 13/16  PASS (81%)
  - 学生管理测试:    2/6   PASS (33%)
  - DDL/DML 测试:    6/6   PASS (100%)
```

## 相关文档

- [PG_UNSUPPORTED_FEATURES.md](PG_UNSUPPORTED_FEATURES.md) - 完整的不兼容特性清单
- [MYSQL_TO_PG_CASES.md](MYSQL_TO_PG_CASES.md) - SQL 转换案例
- [mysql_test_coverage.md](mysql_test_coverage.md) - 测试覆盖详情

## 更新记录

- **2025-11-07**: 版本升级和测试修复
  - go-mysql 升级: v1.7.0 → v1.13.0
  - Go 升级: 1.21 → 1.25.3
  - 测试通过率: 58/65 (89%)
  - 修复的测试:
    - TestComplexQueries_GroupBy: 修复 HAVING 子句占位符转换
    - TestNullValues: 修复 INSERT NULL 值处理
    - TestBatchOperations: 修复 UPDATE 中 CONCAT 函数的括号解析
  - 测试组织:
    - 主测试集: test/integration/ (PostgreSQL 支持的功能)
    - PG 不支持测试集: test/pg-unsupported/ (完全不兼容的特性)
    - 新增 Makefile 目标: make test-pg-unsupported
