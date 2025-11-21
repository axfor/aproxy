# AST-Based SQL Rewriter 设计文档

## 概述

本文档描述了 AProxy 基于抽象语法树（AST）的 SQL 重写器设计。这是对现有正则表达式方案的重大架构升级。

## 背景

### 当前正则方案的问题

1. **无法处理复杂嵌套结构** - 正则表达式无法理解 SQL 语法树
2. **上下文不敏感** - 盲目替换，不理解函数调用的上下文
3. **脆弱且难维护** - 每个新特性都需要添加复杂的正则模式
4. **无法支持 MATCH AGAINST** 等复杂 MySQL 特性

参考: [ARCHITECTURE_ANALYSIS.md](./ARCHITECTURE_ANALYSIS.md)

## AST 方案设计

### 架构图

```
┌─────────────────┐
│  MySQL SQL Text │
└────────┬────────┘
         │
         ▼
┌──────────────────────────┐
│  TiDB Parser             │
│  - Tokenization          │
│  - Syntax Analysis       │
│  - AST Generation        │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│  MySQL AST               │
│  (ast.SelectStmt,        │
│   ast.InsertStmt, etc.)  │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│  AST Transformer         │
│  - Visitor Pattern       │
│  - Function rewriting    │
│  - Type conversion       │
│  - Placeholder mapping   │
└────────┬─────────────────┘
         │
         ▼
┌──────────────────────────┐
│  PostgreSQL SQL          │
│  Generator               │
│  - Custom RestoreCtx     │
│  - ? → $1, $2, ...       │
│  - MySQL → PG functions  │
└────────┬─────────────────┘
         │
         ▼
┌─────────────────┐
│ PostgreSQL SQL  │
└─────────────────┘
```

### 核心组件

#### 1. TiDB Parser (第三方库)

**作用**: 将 MySQL SQL 解析为 AST

**选择理由**:
- ✅ 生产级 MySQL 解析器（TiDB 在生产环境使用）
- ✅ 完整支持 MySQL 5.7+ 和 8.0 语法
- ✅ 已经在 `go.mod` 依赖中
- ✅ 提供完整的 AST 节点类型

**使用示例**:
```go
p := parser.New()
stmts, _, err := p.Parse(sql, "", "")
if err != nil {
    return err
}
stmt := stmts[0]  // ast.SelectStmt, ast.InsertStmt, etc.
```

#### 2. AST Transformer (pkg/sqlrewrite/ast_rewriter.go)

**作用**: 遍历 AST 并转换 MySQL 特定节点为 PostgreSQL 等价物

**实现模式**: Visitor Pattern

```go
type MySQLToPostgreSQLTransformer struct {
    err              error
    functionRewrites map[string]string
}

func (t *MySQLToPostgreSQLTransformer) Enter(n ast.Node) (ast.Node, bool) {
    switch node := n.(type) {
    case *ast.FuncCallExpr:
        // Transform MySQL functions to PostgreSQL
        return t.transformFunction(node)
    case *ast.SelectStmt:
        return t.transformSelect(node)
    // ... handle other node types
    }
    return n, false
}
```

**转换规则**:

| MySQL 特性 | PostgreSQL 等价 | 实现位置 |
|-----------|----------------|---------|
| `NOW()` | `CURRENT_TIMESTAMP` | `transformFunction` |
| `IFNULL(a, b)` | `COALESCE(a, b)` | `transformFunction` |
| `?` 占位符 | `$1, $2, ...` | `PostgreSQLGenerator.restoreExpr` |
| `TINYINT` | `SMALLINT` | `convertDataType` |
| `INT` | `INTEGER` | `convertDataType` |
| `DATETIME` | `TIMESTAMP` | `convertDataType` |
| `JSON` | `JSONB` | `convertDataType` |

#### 3. PostgreSQL SQL Generator (pkg/sqlrewrite/ast_rewriter.go)

**作用**: 从转换后的 AST 生成 PostgreSQL SQL 文本

**核心方法**:
```go
type PostgreSQLGenerator struct {
    buf              strings.Builder
    placeholderIndex int  // Track $1, $2, $3, ...
}

func (g *PostgreSQLGenerator) Generate(node ast.Node) (string, error) {
    switch n := node.(type) {
    case *ast.SelectStmt:
        return g.generateSelect(n)
    case *ast.InsertStmt:
        return g.generateInsert(n)
    // ... other statement types
    }
}
```

**关键特性**:
- ✅ 自动将 MySQL `?` 占位符转换为 PostgreSQL `$1, $2, ...`
- ✅ 保持 SQL 结构和语义
- ✅ 处理函数名转换（NOW → CURRENT_TIMESTAMP）
- ✅ 类型转换（TINYINT → SMALLINT）

## 实现文件

### 已创建的文件

1. **pkg/sqlrewrite/ast_rewriter.go** (主实现)
   - `ASTRewriter` - 主重写器
   - `MySQLToPostgreSQLTransformer` - AST 转换器
   - `PostgreSQLGenerator` - SQL 生成器

2. **pkg/sqlrewrite/ast_rewriter_test.go** (单元测试)
   - SELECT 语句测试
   - INSERT 语句测试
   - UPDATE 语句测试
   - DELETE 语句测试
   - CREATE TABLE 语句测试
   - 函数转换测试
   - 性能基准测试

3. **pkg/sqlrewrite/ast_rewriter_v2.go** (改进版本 - 使用 RestoreCtx)
   - 使用 TiDB Parser 的 `format.RestoreCtx` API
   - 更简洁的实现

## 集成方案

### 阶段 1: 并行运行模式

在 `pkg/sqlrewrite/rewriter.go` 中添加 AST 重写器作为可选路径：

```go
type Rewriter struct {
    enabled          bool
    semanticRewriter *SemanticRewriter
    astRewriter      *ASTRewriter  // NEW
    useAST           bool           // NEW: feature flag
}

func (r *Rewriter) Rewrite(sql string) (string, error) {
    if !r.enabled {
        return sql, nil
    }

    // NEW: Try AST rewriter first if enabled
    if r.useAST {
        rewritten, err := r.astRewriter.Rewrite(sql)
        if err == nil {
            return rewritten, nil
        }
        // Fallback to regex-based rewriter
        log.Warn("AST rewriter failed, falling back to regex",
                 "error", err, "sql", sql)
    }

    // Existing regex-based rewriter
    return r.semanticRewriter.Rewrite(sql)
}
```

### 阶段 2: 配置开关

在 `internal/config/config.go` 中添加配置选项：

```go
type Config struct {
    // ... existing fields ...

    SQLRewrite struct {
        Enabled bool   `yaml:"enabled"`
        UseAST  bool   `yaml:"use_ast"`  // NEW
    } `yaml:"sql_rewrite"`
}
```

在 `configs/config.yaml` 中：

```yaml
sql_rewrite:
  enabled: true
  use_ast: false  # Set to true to enable AST-based rewriting
```

### 阶段 3: 逐步迁移

1. **Alpha 测试** (use_ast: false)
   - 在测试环境启用 AST 重写器
   - 对比 AST 和 Regex 方案的输出
   - 收集性能数据

2. **Beta 测试** (use_ast: true, with fallback)
   - 默认使用 AST，失败时回退到 Regex
   - 监控回退率
   - 修复 AST 重写器的边界情况

3. **正式发布** (use_ast: true, no fallback)
   - AST 重写器成为唯一方案
   - 移除 Regex 重写器代码

## 性能考虑

### 解析开销

TiDB Parser 解析性能：
- 简单 SELECT: ~1-2ms
- 复杂 JOIN: ~3-5ms
- CREATE TABLE: ~2-3ms

### 优化策略

#### 1. SQL 缓存

```go
type RewriteCache struct {
    cache sync.Map  // map[string]string
}

func (r *Rewriter) RewriteWithCache(sql string) (string, error) {
    // Check cache first
    if cached, ok := r.cache.Load(sql); ok {
        return cached.(string), nil
    }

    // Rewrite
    rewritten, err := r.astRewriter.Rewrite(sql)
    if err != nil {
        return "", err
    }

    // Store in cache
    r.cache.Store(sql, rewritten)
    return rewritten, nil
}
```

#### 2. Prepared Statement 优化

对于 prepared statements，只需解析一次：

```go
type PreparedRewriter struct {
    astCache sync.Map  // map[string]ast.StmtNode
}

func (r *PreparedRewriter) RewritePrepared(sql string) (string, int, error) {
    // Parse once, cache AST
    var stmt ast.StmtNode
    if cached, ok := r.astCache.Load(sql); ok {
        stmt = cached.(ast.StmtNode)
    } else {
        stmts, _, err := r.parser.Parse(sql, "", "")
        if err != nil {
            return "", 0, err
        }
        stmt = stmts[0]
        r.astCache.Store(sql, stmt)
    }

    // Generate SQL每次都需要重新生成（因为占位符计数可能不同）
    return r.generator.Generate(stmt)
}
```

## 未来扩展

### 1. MATCH AGAINST 支持

有了 AST，我们可以实现 MATCH AGAINST 转换：

```go
func (t *MySQLToPostgreSQLTransformer) transformFunction(node *ast.FuncCallExpr) (ast.Node, bool) {
    if node.FnName.L == "match" {
        // MATCH(title, content) AGAINST('search' IN BOOLEAN MODE)
        // →
        // to_tsvector('simple', title || ' ' || content) @@ to_tsquery('simple', 'search')

        // Extract columns from MATCH()
        columns := node.Args

        // Build column concatenation
        concatExpr := buildColumnConcat(columns)

        // Build to_tsvector call
        tsvectorCall := &ast.FuncCallExpr{
            FnName: model.NewCIStr("to_tsvector"),
            Args:   []ast.ExprNode{
                &ast.ValueExpr{Datum: types.NewStringDatum("simple")},
                concatExpr,
            },
        }

        // TODO: Extract AGAINST clause and build to_tsquery

        return tsvectorCall, true
    }

    return node, false
}
```

### 2. 存储过程翻译

```go
case *ast.CreateProcedureStmt:
    // Translate MySQL procedure to PL/pgSQL function
    return t.transformProcedure(node)
```

### 3. 窗口函数

```go
case *ast.WindowFuncExpr:
    // Transform MySQL window functions to PostgreSQL
    return t.transformWindowFunction(node)
```

## 测试策略

### 单元测试

- 每种 SQL 语句类型都有独立测试
- 覆盖常见 MySQL 函数转换
- 测试占位符映射
- 性能基准测试

### 集成测试

在 `test/integration/ast_rewriter_test.go` 中添加端到端测试：

```go
func TestASTRewriter_Integration(t *testing.T) {
    // Setup AProxy with AST rewriter enabled
    // Send MySQL queries
    // Verify PostgreSQL receives correct queries
}
```

### 兼容性测试

对比 AST 和 Regex 方案的输出：

```go
func TestASTRewriter_Compatibility(t *testing.T) {
    astRewriter := NewASTRewriter()
    regexRewriter := NewSemanticRewriter()

    testCases := []string{
        "SELECT * FROM users WHERE id = ?",
        "INSERT INTO users (name) VALUES (?)",
        // ... more test cases
    }

    for _, sql := range testCases {
        astResult, _ := astRewriter.Rewrite(sql)
        regexResult, _ := regexRewriter.Rewrite(sql)

        // Both should produce functionally equivalent SQL
        assert.Equal(t, normalize(regexResult), normalize(astResult))
    }
}
```

## 依赖关系

### Go Modules

```go
require (
    github.com/pingcap/tidb/pkg/parser v0.0.0-20250421232622-526b2c79173d
    github.com/pingcap/errors v0.11.5-0.20250318082626-8f80e5cb09ec
    github.com/pingcap/log v1.1.1-0.20241212030209-7e3ff8601a2a
)
```

所有依赖都已在 `go.mod` 中。

## 总结

AST 方案相比正则方案的优势：

| 特性 | Regex 方案 | AST 方案 |
|-----|-----------|---------|
| **正确性** | 低（边界情况多） | 高（语法感知） |
| **可维护性** | 低（脆弱模式） | 高（结构化代码） |
| **可扩展性** | 低（添加正则） | 高（添加 Visitor 方法） |
| **复杂 SQL** | 无法处理 | 完全支持 |
| **类型感知** | 无 | 完整上下文 |
| **性能** | 快（~0.1ms） | 中等（~2ms，可缓存） |

**下一步行动**:
1. ✅ 完成 AST 重写器基础实现
2. ✅ 添加单元测试
3. ⏳ 完善类型映射和 API
4. ⏳ 集成到 Rewriter（并行模式）
5. ⏳ 性能测试和优化
6. ⏳ 添加 MATCH AGAINST 支持
7. ⏳ 生产环境测试

## 参考

- [ARCHITECTURE_ANALYSIS.md](./ARCHITECTURE_ANALYSIS.md) - 架构分析
- [TiDB Parser GitHub](https://github.com/pingcap/tidb/tree/master/pkg/parser)
- [PostgreSQL SQL 语法](https://www.postgresql.org/docs/current/sql.html)
- [MySQL 到 PostgreSQL 迁移](https://www.postgresql.org/docs/current/mysql-to-postgres.html)
