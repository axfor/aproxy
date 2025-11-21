# TiDB Parser 最佳实践 - 从 TiDB 源码学习

本文档总结了从 TiDB 源码中学到的 Parser 使用最佳实践，以及如何应用到 AProxy 项目中。

## 一、核心设计理念

### TiDB 的 SQL 处理流程

```
MySQL SQL
  ↓
Parser.Parse() → AST
  ↓
Preprocessor (语义验证、名称解析)
  ↓
Planner (构建执行计划)
  ↓
Optimizer (优化执行计划)
  ↓
Executor (执行)
```

### AProxy 的对应流程

```
MySQL SQL
  ↓
Parser.Parse() → MySQL AST
  ↓
Visitor (MySQL → PostgreSQL 转换)
  ↓
Restore (生成 PostgreSQL SQL)
  ↓
发送给 PostgreSQL
```

## 二、Parser 初始化和复用

### TiDB 的做法

TiDB **不**为每个 SQL 创建新的 Parser 实例，而是：

```go
// TiDB session 结构
type session struct {
    parser    *parser.Parser  // 复用 parser 实例
    mu        sync.RWMutex    // 保护并发访问
    // ... 其他字段
}

// 解析 SQL
func (s *session) ParseSQL(ctx context.Context, sql string) ([]ast.StmtNode, error) {
    s.mu.Lock()
    defer s.mu.Unlock()

    return s.parser.Parse(sql, "", "")
}
```

**关键点**:
- Parser 对象**不是线程安全**的
- Parser 对象**不是轻量级**的（包含大量状态）
- 应该在 session/connection 级别复用

### AProxy 的改进方案

```go
// pkg/sqlrewrite/rewriter.go
type Rewriter struct {
    enabled          bool
    semanticRewriter *SemanticRewriter
    astRewriter      *ASTRewriter      // NEW
    useAST           bool               // NEW: 配置开关
    mu               sync.Mutex         // NEW: 保护并发访问
}

func NewRewriter(enabled bool, useAST bool) *Rewriter {
    return &Rewriter{
        enabled:          enabled,
        semanticRewriter: NewSemanticRewriter(),
        astRewriter:      NewASTRewriter(),  // 初始化一次
        useAST:           useAST,
    }
}

func (r *Rewriter) Rewrite(sql string) (string, error) {
    if !r.enabled {
        return sql, nil
    }

    // 如果启用 AST 方案
    if r.useAST {
        r.mu.Lock()
        defer r.mu.Unlock()

        rewritten, err := r.astRewriter.Rewrite(sql)
        if err != nil {
            // 记录错误，回退到 Regex 方案
            log.Warn("AST rewriter failed, falling back to regex",
                     "error", err, "sql", sql)
            return r.semanticRewriter.Rewrite(sql)
        }
        return rewritten, nil
    }

    // 默认使用 Regex 方案
    return r.semanticRewriter.Rewrite(sql)
}
```

## 三、Visitor 模式深度应用

### TiDB Preprocessor 的实现

TiDB 的 `preprocessor` 是一个完美的 Visitor 模式示例：

```go
// planner/core/preprocess.go
type preprocessor struct {
    ctx                 sessionctx.Context
    err                 error
    flag                preprocessorFlag
    stmtTp              byte
    tableAliasInJoin    []map[string]interface{}
    withName            map[string]interface{}
}

func (p *preprocessor) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
    switch node := in.(type) {
    case *ast.SelectStmt:
        // 1. 记录语句类型
        p.stmtTp = TypeSelect

        // 2. 验证字段
        if node.Fields == nil {
            p.err = errors.New("SELECT must have fields")
            return in, true  // 跳过子节点
        }

        // 3. 处理 WITH 子句
        if node.With != nil {
            p.handleWith(node.With)
        }

    case *ast.TableName:
        // 解析表名，验证是否存在
        schema := node.Schema.L
        table := node.Name.L

        // 查询元数据
        _, err := p.ctx.GetDomain().InfoSchema().TableByName(schema, table)
        if err != nil {
            p.err = fmt.Errorf("table %s.%s not found", schema, table)
        }

    case *ast.ColumnName:
        // 解析列名，后续会进行类型推断
        // ...
    }

    return in, false
}

func (p *preprocessor) Leave(in ast.Node) (out ast.Node, ok bool) {
    switch node := in.(type) {
    case *ast.SelectStmt:
        // 清理 WITH 子句的临时状态
        if node.With != nil {
            for _, cte := range node.With.CTEs {
                delete(p.withName, cte.Name.L)
            }
        }
    }

    return in, p.err == nil
}
```

**关键技巧**:
1. **状态管理**: 使用 struct 字段跟踪遍历状态
2. **错误处理**: 将错误保存在 visitor 中，而不是立即返回
3. **跳过子节点**: 通过 `skipChildren = true` 控制遍历深度
4. **清理逻辑**: 在 `Leave` 中清理临时状态

### AProxy 的 Visitor 实现

```go
// pkg/sqlrewrite/ast_visitor.go
type MySQLToPostgreSQLVisitor struct {
    err              error
    functionRewrites map[string]string
    typeConversions  map[byte]string

    // 状态跟踪
    inCreateTable    bool
    currentTableName string
    placeholderIndex int
}

func NewMySQLToPostgreSQLVisitor() *MySQLToPostgreSQLVisitor {
    return &MySQLToPostgreSQLVisitor{
        functionRewrites: map[string]string{
            "now":            "current_timestamp",
            "ifnull":         "coalesce",
            "unix_timestamp": "extract_epoch",
            "curdate":        "current_date",
            "curtime":        "current_time",
        },
        typeConversions: map[byte]string{
            mysql.TypeTiny:      "SMALLINT",
            mysql.TypeShort:     "SMALLINT",
            mysql.TypeLong:      "INTEGER",
            mysql.TypeLonglong:  "BIGINT",
            mysql.TypeFloat:     "REAL",
            mysql.TypeDouble:    "DOUBLE PRECISION",
            mysql.TypeDatetime:  "TIMESTAMP",
            mysql.TypeJSON:      "JSONB",
        },
    }
}

func (v *MySQLToPostgreSQLVisitor) Enter(n ast.Node) (ast.Node, bool) {
    switch node := n.(type) {
    case *ast.CreateTableStmt:
        v.inCreateTable = true
        v.currentTableName = node.Table.Name.O
        return node, false

    case *ast.ColumnDef:
        if v.inCreateTable {
            // 转换数据类型
            return v.transformColumnDef(node), false
        }

    case *ast.FuncCallExpr:
        // 转换函数名
        funcName := strings.ToLower(node.FnName.L)
        if pgFunc, ok := v.functionRewrites[funcName]; ok {
            node.FnName.L = pgFunc
            node.FnName.O = strings.ToUpper(pgFunc)
        }
        return node, false
    }

    return n, false
}

func (v *MySQLToPostgreSQLVisitor) Leave(n ast.Node) (ast.Node, bool) {
    switch node := n.(type) {
    case *ast.CreateTableStmt:
        v.inCreateTable = false
        v.currentTableName = ""
    }

    return n, v.err == nil
}

func (v *MySQLToPostgreSQLVisitor) transformColumnDef(col *ast.ColumnDef) *ast.ColumnDef {
    // 获取 MySQL 类型
    mysqlType := col.Tp.GetType()

    // 转换为 PostgreSQL 类型
    if pgType, ok := v.typeConversions[mysqlType]; ok {
        // 注意: 这里不能直接修改 col.Tp.Tp
        // 因为 FieldType 可能被多个地方引用
        // 应该创建新的 FieldType
        newTp := *col.Tp
        col.Tp = &newTp

        // 设置 PostgreSQL 类型名称（通过 Restore 时体现）
        // 实际实现需要在 Restore 阶段处理
    }

    return col
}
```

## 四、Restore 机制详解

### TiDB 的 Restore 实现

每个 AST 节点都实现了 `Restore` 方法：

```go
// ast/ddl.go - DropDatabaseStmt 的实现
func (n *DropDatabaseStmt) Restore(ctx *format.RestoreCtx) error {
    ctx.WriteKeyWord("DROP DATABASE ")
    if n.IfExists {
        ctx.WriteKeyWord("IF EXISTS ")
    }
    ctx.WriteName(n.Name)
    return nil
}

// ast/dml.go - SelectStmt 的实现
func (n *SelectStmt) Restore(ctx *format.RestoreCtx) error {
    ctx.WriteKeyWord("SELECT ")

    if n.Distinct {
        ctx.WriteKeyWord("DISTINCT ")
    }

    // Field list
    if err := n.Fields.Restore(ctx); err != nil {
        return err
    }

    // FROM clause
    if n.From != nil {
        ctx.WriteKeyWord(" FROM ")
        if err := n.From.Restore(ctx); err != nil {
            return err
        }
    }

    // WHERE clause
    if n.Where != nil {
        ctx.WriteKeyWord(" WHERE ")
        if err := n.Where.Restore(ctx); err != nil {
            return err
        }
    }

    // ... 其他子句

    return nil
}
```

**RestoreCtx 的核心方法**:

```go
type RestoreCtx struct {
    Flags RestoreFlags
    In    io.Writer
}

// 写入关键字 (根据 Flags 决定大小写)
func (ctx *RestoreCtx) WriteKeyWord(s string)

// 写入标识符 (根据 Flags 决定是否加引号)
func (ctx *RestoreCtx) WriteName(s string)

// 写入普通文本
func (ctx *RestoreCtx) WritePlain(s string)

// 写入字符串字面量 (根据 Flags 决定引号类型)
func (ctx *RestoreCtx) WriteString(s string)
```

### AProxy 的 Restore 策略

由于我们需要生成 PostgreSQL SQL，不能直接使用 TiDB 的 Restore（它生成的是 MySQL SQL）。

**方案 1: 自定义 RestoreCtx**

```go
// PostgreSQL 专用的 RestoreCtx
type PostgreSQLRestoreCtx struct {
    buf              *strings.Builder
    placeholderIndex int
    flags            format.RestoreFlags
}

func (ctx *PostgreSQLRestoreCtx) WriteKeyWord(s string) {
    ctx.buf.WriteString(strings.ToUpper(s))
}

func (ctx *PostgreSQLRestoreCtx) WriteName(s string) {
    // PostgreSQL 使用双引号，而不是反引号
    ctx.buf.WriteString(`"`)
    ctx.buf.WriteString(s)
    ctx.buf.WriteString(`"`)
}

func (ctx *PostgreSQLRestoreCtx) WritePlaceholder() {
    ctx.buf.WriteString(fmt.Sprintf("$%d", ctx.placeholderIndex))
    ctx.placeholderIndex++
}
```

**方案 2: 遍历 AST 手动生成**

这是我们当前的方案，更灵活：

```go
func (g *PostgreSQLGenerator) Generate(node ast.Node) (string, error) {
    g.buf.Reset()
    g.placeholderIndex = 1

    switch n := node.(type) {
    case *ast.SelectStmt:
        return g.generateSelect(n)
    case *ast.InsertStmt:
        return g.generateInsert(n)
    // ...
    }
}

func (g *PostgreSQLGenerator) generateSelect(node *ast.SelectStmt) (string, error) {
    g.buf.WriteString("SELECT ")

    // Fields
    for i, field := range node.Fields.Fields {
        if i > 0 {
            g.buf.WriteString(", ")
        }
        if err := g.generateExpr(field.Expr); err != nil {
            return "", err
        }
    }

    // FROM
    if node.From != nil {
        g.buf.WriteString(" FROM ")
        if err := g.generateTableRefs(node.From.TableRefs); err != nil {
            return "", err
        }
    }

    // WHERE
    if node.Where != nil {
        g.buf.WriteString(" WHERE ")
        if err := g.generateExpr(node.Where); err != nil {
            return "", err
        }
    }

    return g.buf.String(), nil
}
```

## 五、类型系统使用

### TiDB 的类型常量

```go
// pkg/parser/mysql/type.go
const (
    TypeTiny       byte = 1    // TINYINT
    TypeShort      byte = 2    // SMALLINT
    TypeLong       byte = 3    // INT
    TypeFloat      byte = 4    // FLOAT
    TypeDouble     byte = 5    // DOUBLE
    TypeNull       byte = 6    // NULL
    TypeTimestamp  byte = 7    // TIMESTAMP
    TypeLonglong   byte = 8    // BIGINT
    TypeInt24      byte = 9    // MEDIUMINT
    TypeDate       byte = 10   // DATE
    TypeDuration   byte = 11   // TIME
    TypeDatetime   byte = 12   // DATETIME
    TypeYear       byte = 13   // YEAR
    TypeNewDate    byte = 14   // DATE (internal)
    TypeVarchar    byte = 15   // VARCHAR
    TypeBit        byte = 16   // BIT
    TypeJSON       byte = 245  // JSON
    TypeNewDecimal byte = 246  // DECIMAL
    TypeEnum       byte = 247  // ENUM
    TypeSet        byte = 248  // SET
    TypeTinyBlob   byte = 249  // TINYBLOB
    TypeMediumBlob byte = 250  // MEDIUMBLOB
    TypeLongBlob   byte = 251  // LONGBLOB
    TypeBlob       byte = 252  // BLOB
    TypeVarString  byte = 253  // VAR_STRING
    TypeString     byte = 254  // STRING
    TypeGeometry   byte = 255  // GEOMETRY
)
```

### AProxy 的类型映射表

```go
// pkg/sqlrewrite/type_mapping.go
package sqlrewrite

import (
    "fmt"
    "github.com/pingcap/tidb/pkg/parser/mysql"
    "github.com/pingcap/tidb/pkg/parser/types"
)

type TypeMapper struct{}

func NewTypeMapper() *TypeMapper {
    return &TypeMapper{}
}

func (m *TypeMapper) MySQLToPostgreSQL(tp *types.FieldType) string {
    mysqlType := tp.GetType()

    // 处理 UNSIGNED
    isUnsigned := mysql.HasUnsignedFlag(tp.GetFlag())

    switch mysqlType {
    case mysql.TypeTiny:
        // TINYINT -> SMALLINT
        // TINYINT UNSIGNED -> SMALLINT (PostgreSQL 没有 UNSIGNED)
        return "SMALLINT"

    case mysql.TypeShort:
        if isUnsigned {
            return "INTEGER" // SMALLINT UNSIGNED 可能溢出，用 INTEGER
        }
        return "SMALLINT"

    case mysql.TypeLong, mysql.TypeInt24:
        if isUnsigned {
            return "BIGINT" // INT UNSIGNED 可能溢出，用 BIGINT
        }
        return "INTEGER"

    case mysql.TypeLonglong:
        return "BIGINT"

    case mysql.TypeFloat:
        return "REAL"

    case mysql.TypeDouble:
        return "DOUBLE PRECISION"

    case mysql.TypeNewDecimal:
        flen := tp.GetFlen()
        decimal := tp.GetDecimal()
        if flen > 0 {
            return fmt.Sprintf("NUMERIC(%d,%d)", flen, decimal)
        }
        return "NUMERIC"

    case mysql.TypeVarchar, mysql.TypeVarString:
        flen := tp.GetFlen()
        if flen > 0 {
            return fmt.Sprintf("VARCHAR(%d)", flen)
        }
        return "VARCHAR"

    case mysql.TypeString:
        flen := tp.GetFlen()
        if flen > 0 {
            return fmt.Sprintf("CHAR(%d)", flen)
        }
        return "CHAR"

    case mysql.TypeTinyBlob, mysql.TypeBlob,
         mysql.TypeMediumBlob, mysql.TypeLongBlob:
        return "BYTEA"

    case mysql.TypeDate:
        return "DATE"

    case mysql.TypeDatetime, mysql.TypeTimestamp:
        return "TIMESTAMP"

    case mysql.TypeDuration:
        return "TIME"

    case mysql.TypeYear:
        return "SMALLINT"

    case mysql.TypeJSON:
        return "JSONB"

    case mysql.TypeEnum:
        // 可以创建 ENUM 类型或使用 VARCHAR
        return "VARCHAR(50)"

    case mysql.TypeSet:
        // PostgreSQL 使用数组
        return "TEXT[]"

    case mysql.TypeBit:
        flen := tp.GetFlen()
        if flen > 0 {
            return fmt.Sprintf("BIT(%d)", flen)
        }
        return "BIT"

    default:
        // 未知类型，默认使用 TEXT
        return "TEXT"
    }
}
```

## 六、占位符处理的正确方式

### TiDB 的占位符处理

```go
// pkg/types/parser_driver/value_expr.go
type ParamMarkerExpr struct {
    ValueExpr
    Offset    int
    Order     int
    InExecute bool
}

func (n *ParamMarkerExpr) Restore(ctx *format.RestoreCtx) error {
    ctx.WritePlain("?")
    return nil
}
```

### AProxy 的占位符转换

MySQL 使用 `?`，PostgreSQL 使用 `$1, $2, ...`

**方法 1: 在 Visitor 中记录位置**

```go
type PlaceholderMapper struct {
    positions []int  // 记录占位符位置
    count     int
}

func (v *MySQLToPostgreSQLVisitor) Enter(n ast.Node) (ast.Node, bool) {
    switch node := n.(type) {
    case *driver.ParamMarkerExpr:
        // 记录位置
        v.placeholderMapper.positions = append(v.placeholderMapper.positions, v.currentPosition)
        v.placeholderMapper.count++
    }
    return n, false
}
```

**方法 2: 在 Generator 中转换** (推荐)

```go
func (g *PostgreSQLGenerator) generateExpr(node ast.ExprNode) error {
    switch n := node.(type) {
    case *driver.ParamMarkerExpr:
        // 直接转换为 $N
        g.buf.WriteString(fmt.Sprintf("$%d", g.placeholderIndex))
        g.placeholderIndex++
        return nil
    }
}
```

## 七、错误处理和日志

### TiDB 的错误处理模式

```go
func (p *preprocessor) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
    // ... 处理逻辑

    if someError {
        p.err = errors.Trace(someError)  // 包装错误
        return in, true  // 跳过子节点
    }

    return in, false
}

func (p *preprocessor) Leave(in ast.Node) (out ast.Node, ok bool) {
    return in, p.err == nil  // 通过 ok 传播错误
}

// 使用
preprocessor := &preprocessor{ctx: ctx}
node.Accept(preprocessor)
if preprocessor.err != nil {
    return errors.Trace(preprocessor.err)
}
```

### AProxy 的改进建议

```go
func (r *ASTRewriter) Rewrite(sql string) (string, error) {
    // 1. Parse
    stmts, _, err := r.parser.Parse(sql, "", "")
    if err != nil {
        return "", fmt.Errorf("failed to parse SQL %q: %w", sql, err)
    }

    // 2. Transform
    visitor := NewMySQLToPostgreSQLVisitor()
    stmts[0].Accept(visitor)
    if visitor.err != nil {
        return "", fmt.Errorf("failed to transform AST for SQL %q: %w", sql, visitor.err)
    }

    // 3. Generate
    generator := NewPostgreSQLGenerator()
    pgSQL, err := generator.Generate(stmts[0])
    if err != nil {
        return "", fmt.Errorf("failed to generate PostgreSQL SQL from %q: %w", sql, err)
    }

    // 4. 日志记录
    log.Debug("SQL rewritten",
        "original", sql,
        "rewritten", pgSQL,
        "placeholder_count", generator.placeholderIndex - 1)

    return pgSQL, nil
}
```

## 八、性能优化技巧

### 1. Parser 实例池

```go
var parserPool = sync.Pool{
    New: func() interface{} {
        return parser.New()
    },
}

func (r *ASTRewriter) Rewrite(sql string) (string, error) {
    p := parserPool.Get().(*parser.Parser)
    defer parserPool.Put(p)

    stmts, _, err := p.Parse(sql, "", "")
    // ...
}
```

### 2. SQL 缓存

```go
type RewriteCache struct {
    cache *lru.Cache  // 使用 LRU 缓存
    mu    sync.RWMutex
}

func (r *Rewriter) Rewrite(sql string) (string, error) {
    // 检查缓存
    r.cache.mu.RLock()
    if cached, ok := r.cache.cache.Get(sql); ok {
        r.cache.mu.RUnlock()
        return cached.(string), nil
    }
    r.cache.mu.RUnlock()

    // 执行重写
    rewritten, err := r.astRewriter.Rewrite(sql)
    if err != nil {
        return "", err
    }

    // 存入缓存
    r.cache.mu.Lock()
    r.cache.cache.Add(sql, rewritten)
    r.cache.mu.Unlock()

    return rewritten, nil
}
```

### 3. 避免不必要的字符串拷贝

```go
// 不好的做法
func (g *Generator) generateName(name string) string {
    return `"` + name + `"`
}

// 好的做法
func (g *Generator) generateName(name string) {
    g.buf.WriteString(`"`)
    g.buf.WriteString(name)
    g.buf.WriteString(`"`)
}
```

## 九、测试策略

### 1. 单元测试

```go
func TestTypeConversion(t *testing.T) {
    mapper := NewTypeMapper()

    tests := []struct {
        name     string
        mysqlType byte
        expected string
    }{
        {"TINYINT", mysql.TypeTiny, "SMALLINT"},
        {"INT", mysql.TypeLong, "INTEGER"},
        {"BIGINT", mysql.TypeLonglong, "BIGINT"},
        {"DATETIME", mysql.TypeDatetime, "TIMESTAMP"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tp := &types.FieldType{}
            tp.SetType(tt.mysqlType)

            result := mapper.MySQLToPostgreSQL(tp)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### 2. 集成测试

```go
func TestAST_EndToEnd(t *testing.T) {
    rewriter := NewASTRewriter()

    tests := []struct {
        mysql    string
        postgres string
    }{
        {
            "SELECT * FROM users WHERE id = ?",
            "SELECT * FROM users WHERE id = $1",
        },
        {
            "SELECT NOW(), IFNULL(name, 'Unknown') FROM users",
            "SELECT CURRENT_TIMESTAMP, COALESCE(name, 'Unknown') FROM users",
        },
    }

    for _, tt := range tests {
        result, err := rewriter.Rewrite(tt.mysql)
        require.NoError(t, err)
        assert.Equal(t, tt.postgres, result)
    }
}
```

### 3. 模糊测试

```go
func FuzzAST(f *testing.F) {
    rewriter := NewASTRewriter()

    f.Add("SELECT * FROM t")
    f.Add("INSERT INTO t VALUES (1)")

    f.Fuzz(func(t *testing.T, sql string) {
        _, err := rewriter.Rewrite(sql)
        // 不应该 panic
        if err != nil {
            t.Logf("Failed to rewrite: %v", err)
        }
    })
}
```

## 十、总结

### 从 TiDB 学到的关键点

1. **Parser 复用**: 不要为每个 SQL 创建新 Parser
2. **Visitor 模式**: 分离遍历逻辑和转换逻辑
3. **错误传播**: 在 Visitor 中累积错误，最后统一处理
4. **类型系统**: 使用 `types.FieldType` 和 `mysql.Type*` 常量
5. **Restore 机制**: 理解 TiDB 的 Restore，但为 PostgreSQL 自定义实现
6. **状态管理**: 在 Visitor 中维护必要的状态信息

### AProxy 的实施建议

**短期（1-2 周）**:
- ✅ 完善类型映射表
- ✅ 实现完整的 Visitor
- ✅ 添加单元测试

**中期（2-4 周）**:
- ✅ 集成到 Rewriter，添加配置开关
- ✅ 性能优化（缓存、池化）
- ✅ 集成测试和兼容性测试

**长期（1-3 个月）**:
- ✅ MATCH AGAINST 支持
- ✅ 存储过程翻译
- ✅ 复杂查询优化
- ✅ 生产环境部署

---

**参考资源**:
- [TiDB Parser GitHub](https://github.com/pingcap/tidb/tree/master/pkg/parser)
- [TiDB Preprocessor](https://github.com/pingcap/tidb/blob/master/planner/core/preprocess.go)
- [TiDB Type System](https://github.com/pingcap/tidb/tree/master/pkg/parser/types)
