# SQL Semantic Rewriting Architecture

## 概述

AProxy采用**完全基于语义规则的SQL重写架构**，以实现100% MySQL到PostgreSQL的兼容性。

## 架构原则

1. **语义优先**：所有SQL转换基于对SQL语义的深度理解，而非简单的文本替换
2. **可扩展性**：通过添加parser规则和semantic规则轻松支持新的MySQL语法
3. **准确性**：避免正则表达式的误匹配，确保SQL转换的正确性
4. **可测试性**：每个转换规则都有对应的单元测试

## 核心组件

### 1. Parser层 (pkg/sqlrewrite/parser.go)

负责将MySQL SQL语句解析为抽象语法树(AST)。

**支持的SQL类型**：
- DDL: CREATE TABLE, ALTER TABLE, DROP TABLE, CREATE INDEX, DROP INDEX
- DML: SELECT, INSERT, UPDATE, DELETE
- TCL: BEGIN, COMMIT, ROLLBACK
- 其他: SHOW, DESCRIBE, USE

**AST结构**：
```go
type Statement interface {
    Type() StatementType
}

type CreateTableStatement struct {
    TableName    string
    Columns      []ColumnDefinition
    Indexes      []IndexDefinition
    Constraints  []Constraint
    // ...
}

type SelectStatement struct {
    Columns      []SelectColumn
    From         []TableReference
    Where        Expression
    GroupBy      []Expression
    Having       Expression
    OrderBy      []OrderByClause
    Limit        *LimitClause
    // ...
}
```

### 2. Semantic层 (pkg/sqlrewrite/semantic.go)

负责将MySQL AST转换为PostgreSQL兼容的SQL。

**转换规则类别**：

#### 2.1 类型转换规则
- `TINYINT` → `SMALLINT`
- `INT UNSIGNED` → `BIGINT`
- `DATETIME` → `TIMESTAMP`
- `ENUM` → `VARCHAR` 或 自定义类型
- `AUTO_INCREMENT` → `SERIAL` 或 `IDENTITY`

#### 2.2 语法转换规则
- `LIMIT offset, count` → `LIMIT count OFFSET offset`
- `\`backtick\`` → `"doublequote"`
- `INDEX` in CREATE TABLE → 分离的 `CREATE INDEX`
- `IF NOT EXISTS` → PostgreSQL等价语法

#### 2.3 函数转换规则
- `NOW()` → `CURRENT_TIMESTAMP`
- `UNIX_TIMESTAMP()` → `EXTRACT(EPOCH FROM ...)`
- `DATE_FORMAT()` → `TO_CHAR()`
- `CONCAT()` → `||` 或保留 `CONCAT()`

#### 2.4 运算符转换规则
- `<=>` (NULL-safe equal) → `IS NOT DISTINCT FROM`
- `&&` → `AND`
- `||` (MySQL OR) → `OR`

### 3. Rewriter层 (pkg/sqlrewrite/rewriter.go)

统一的重写接口和执行协调器。

**职责**：
1. 接收原始SQL
2. 调用Parser解析
3. 根据语句类型选择对应的Semantic转换器
4. 返回转换后的SQL和可能的附加语句
5. 提供缓存机制提升性能

**接口**：
```go
type Rewriter interface {
    Rewrite(sql string) (string, error)
    GetAdditionalStatements() []string
    ClearAdditionalStatements()
}
```

## 转换流程

```
MySQL SQL
    ↓
Parser.Parse()
    ↓
AST (Abstract Syntax Tree)
    ↓
SemanticRewriter.Transform()
    ↓
PostgreSQL SQL + Additional Statements
    ↓
执行引擎
```

## 扩展指南

### 添加新的DDL支持

1. 在 `parser.go` 中添加新的Statement结构和解析函数
2. 在 `semantic.go` 中添加对应的Transform方法
3. 在 `rewriter.go` 中注册新的语句类型路由
4. 添加单元测试

示例：支持 `ALTER TABLE ADD COLUMN`

```go
// 1. parser.go
type AlterTableStatement struct {
    TableName string
    Actions   []AlterAction
}

func ParseAlterTable(sql string) (*AlterTableStatement, error) {
    // 解析逻辑
}

// 2. semantic.go
func (sr *SemanticRewriter) RewriteAlterTable(stmt *AlterTableStatement) (string, []string, error) {
    // 转换逻辑
}

// 3. rewriter.go
case StatementTypeAlterTable:
    return r.semanticRewriter.RewriteAlterTable(stmt.(*AlterTableStatement))
```

### 添加新的函数转换

在 `semantic.go` 的 `convertFunctionCall()` 方法中添加：

```go
func (sr *SemanticRewriter) convertFunctionCall(expr *FunctionCall) string {
    switch strings.ToUpper(expr.Name) {
    case "IFNULL":
        return fmt.Sprintf("COALESCE(%s)", strings.Join(convertArgs(expr.Args), ", "))
    case "YOUR_NEW_FUNCTION":
        // 转换逻辑
    }
}
```

## 性能优化

1. **SQL解析缓存**：相同的SQL只解析一次
2. **预编译模式检测**：识别prepared statement模式，避免重复解析
3. **增量解析**：对于简单SQL，使用快速路径
4. **并发安全**：支持多goroutine并发重写

## 测试策略

1. **单元测试**：每个parser和semantic函数都有独立测试
2. **集成测试**：使用真实的MySQL测试用例库
3. **回归测试**：记录所有bug修复的测试用例
4. **性能测试**：确保重写开销<1ms

## 与现有正则方案的对比

| 维度 | 正则表达式方案 | 语义规则方案 |
|------|----------------|--------------|
| 准确性 | 容易误匹配 | 100%准确 |
| 扩展性 | 难以维护 | 结构化扩展 |
| 性能 | 简单场景快 | 可通过缓存优化 |
| 复杂SQL支持 | 不支持 | 完全支持 |
| 错误处理 | 难以定位 | 精确报错 |
| 100%兼容目标 | 无法达成 | 可以达成 |

## 迁移计划

### 第一阶段：核心DDL（已完成）
- ✅ CREATE TABLE with INDEX
- ✅ 基础类型转换

### 第二阶段：完整DDL
- ALTER TABLE
- DROP TABLE
- CREATE/DROP INDEX
- CREATE/DROP DATABASE

### 第三阶段：DML语句
- SELECT (包括子查询、JOIN)
- INSERT
- UPDATE
- DELETE

### 第四阶段：高级特性
- 存储过程转换
- 触发器转换
- 视图转换
- 事务控制语句

### 第五阶段：MySQL特定功能
- SHOW语句
- DESCRIBE语句
- 系统变量模拟
- 信息模式(information_schema)

## 结论

全语义规则架构是实现100% MySQL到PostgreSQL兼容性的**唯一可行路径**。虽然初期开发投入较大，但长期维护成本远低于正则表达式方案，且能提供更好的用户体验和更高的可靠性。
