# AProxy Architecture Analysis: Regex vs AST-Based SQL Rewriting

## Current Implementation

AProxy currently uses a **regex-based SQL parsing and rewriting** approach implemented across three main files:

### 1. [`pkg/sqlrewrite/parser.go`](../pkg/sqlrewrite/parser.go)

**Approach**: Uses 40+ pre-compiled regular expressions to parse SQL statements

**Examples**:
```go
createTableRe   = regexp.MustCompile(`(?is)CREATE\s+TABLE\s+...`)
insertTableRe   = regexp.MustCompile(`(?i)INSERT\s+INTO\s+...`)
selectColumnsRe = regexp.MustCompile(`(?i)SELECT\s+(.+?)(?:\s+FROM|...)`)
```

**Manual parsing**: Uses custom functions like `splitRespectingParens()` to handle nested structures:
```go
func splitRespectingParens(s string) []string {
    var parts []string
    parenLevel := 0
    // Manual character-by-character parsing
    for i, ch := range s { ... }
}
```

### 2. [`pkg/sqlrewrite/rewriter.go`](../pkg/sqlrewrite/rewriter.go)

**Approach**: Routes statements based on string prefix matching

```go
func (r *Rewriter) Rewrite(sql string) (string, error) {
    upper := strings.ToUpper(sql)

    switch {
    case strings.HasPrefix(upper, "CREATE TABLE"):
        return r.rewriteCreateTable(sql)
    case strings.HasPrefix(upper, "INSERT"):
        return r.rewriteInsert(sql)
    // ... hardcoded routing for each statement type
    }
}
```

### 3. [`pkg/sqlrewrite/semantic.go`](../pkg/sqlrewrite/semantic.go)

**Approach**: Uses more regex for expression and function call conversion

```go
func (sr *SemanticRewriter) convertFunctionCalls(expr string) string {
    // NOW() -> CURRENT_TIMESTAMP AT TIME ZONE offset
    expr = regexp.MustCompile(`(?i)\bNOW\(\s*\)`).ReplaceAllString(expr, nowReplacement)

    // IFNULL(a, b) -> COALESCE(a, b)
    expr = regexp.MustCompile(`(?i)\bIFNULL\(`).ReplaceAllString(expr, "COALESCE(")

    // ... more regex replacements
}
```

## Limitations of Regex-Based Approach

### 1. **Cannot Handle Complex Nested Structures**

Example: `MATCH AGAINST` full-text search
```sql
-- MySQL
SELECT title
FROM test_fulltext
WHERE MATCH(title, content) AGAINST('+MySQL -Oracle' IN BOOLEAN MODE)
```

**Why regex fails**:
- Need to identify `MATCH()` function in WHERE clause context
- Extract column list: `title, content`
- Parse `AGAINST()` clause with mode flag: `IN BOOLEAN MODE`
- Map boolean operators: `+MySQL -Oracle` → `MySQL & !Oracle`
- Generate column concatenation: `title || ' ' || content`
- Build PostgreSQL equivalent: `to_tsvector('simple', title || ' ' || content) @@ to_tsquery('simple', 'MySQL & !Oracle')`

This transformation requires:
- Context awareness (WHERE clause vs SELECT clause)
- Nested function parsing
- Operator precedence understanding
- Type coercion rules

**Regex cannot do this reliably**.

### 2. **Context-Insensitive Transformations**

Regex replacements don't understand SQL context:

```sql
-- This regex transformation is wrong:
SELECT NOW() AS creation_date  -- Should use NOW()
WHERE created_at = NOW()       -- Should use NOW()
HAVING COUNT(*) > NOW()        -- ERROR: NOW() used in wrong context
```

Regex blindly replaces all `NOW()` without understanding whether it's syntactically valid in that position.

### 3. **Brittle String Prefix Routing**

Current routing:
```go
case strings.HasPrefix(upper, "SELECT"):
    return r.rewriteSelect(sql)
```

**Problems**:
- Cannot handle: `WITH cte AS (SELECT ...) SELECT ...` (CTE)
- Cannot handle: `(SELECT ...) UNION (SELECT ...)` (subqueries)
- Cannot detect MySQL-specific statements that don't fit prefix patterns

### 4. **No Semantic Understanding**

The rewriter cannot:
- Validate SQL correctness
- Understand column types
- Apply type-specific transformations
- Handle operator precedence
- Detect unsupported MySQL features proactively

### 5. **Maintenance Burden**

Every new MySQL feature requires:
- Adding new regex patterns (error-prone)
- Updating manual parsing logic
- Testing edge cases extensively
- Risk of breaking existing patterns

## Recommended: AST-Based Architecture

### Architecture Overview

```
┌─────────────────┐
│  MySQL SQL Text │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────┐
│  SQL Parser (TiDB Parser)   │
│  - Tokenization             │
│  - Syntax Analysis          │
│  - AST Generation           │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  MySQL AST                  │
│  (Abstract Syntax Tree)     │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  AST Transformer            │
│  - Node-by-node traversal   │
│  - MySQL→PostgreSQL mapping │
│  - Type conversion          │
│  - Function rewriting       │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  PostgreSQL AST             │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  SQL Generator              │
│  - AST → SQL text           │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│  PostgreSQL SQL Text        │
└─────────────────────────────┘
```

### Implementation Approach

#### 1. Use Existing SQL Parser

**Recommended**: [TiDB Parser](https://github.com/pingcap/tidb/tree/master/pkg/parser)
- Already mentioned in README.md Related Projects
- Battle-tested MySQL SQL parser
- Generates full AST
- Handles all MySQL syntax including edge cases

**Example**:
```go
import (
    "github.com/pingcap/tidb/pkg/parser"
    "github.com/pingcap/tidb/pkg/parser/ast"
)

// Parse MySQL SQL
p := parser.New()
stmts, err := p.Parse(mysqlSQL, "", "")

// Get AST
stmt := stmts[0]
```

#### 2. Implement AST Visitor Pattern

```go
type MySQLToPostgreSQLVisitor struct {
    ast.Visitor
    // Store transformation state
}

func (v *MySQLToPostgreSQLVisitor) Enter(n ast.Node) (ast.Node, bool) {
    switch node := n.(type) {
    case *ast.SelectStmt:
        return v.transformSelect(node)
    case *ast.FuncCallExpr:
        return v.transformFunction(node)
    case *ast.BinaryOperationExpr:
        return v.transformOperator(node)
    // ... handle all node types
    }
    return n, false
}
```

#### 3. Transform Specific MySQL Features

**Example: MATCH AGAINST transformation**

```go
func (v *MySQLToPostgreSQLVisitor) transformFunction(node *ast.FuncCallExpr) (ast.Node, bool) {
    if node.FnName.L == "match" {
        // Extract columns from MATCH(col1, col2, ...)
        columns := node.Args

        // Find AGAINST clause in WHERE context
        againstExpr := findAgainstClause(node.Parent)

        // Build: to_tsvector('simple', col1 || ' ' || col2)
        tsvectorCall := buildTsVectorCall(columns)

        // Build: to_tsquery('simple', 'search & terms')
        tsqueryCall := buildTsQueryCall(againstExpr)

        // Build: tsvector @@ tsquery
        return &ast.BinaryOperationExpr{
            Op: opcode.FullTextMatch,
            L:  tsvectorCall,
            R:  tsqueryCall,
        }, true
    }

    // Handle other MySQL functions
    return v.transformMySQLFunction(node)
}
```

#### 4. Generate PostgreSQL SQL

Use TiDB's `ast.RestoreCtx` or implement custom SQL generator:

```go
func GeneratePostgreSQLSQL(node ast.Node) (string, error) {
    var buf strings.Builder
    ctx := &RestoreCtx{
        Writer: &buf,
        Flags:  PostgreSQLFlags,
    }
    err := node.Restore(ctx)
    return buf.String(), err
}
```

### Benefits of AST Approach

| Feature | Regex-Based | AST-Based |
|---------|-------------|-----------|
| **Correctness** | Low (many edge cases) | High (syntax-aware) |
| **Maintainability** | Low (brittle patterns) | High (structured code) |
| **Extensibility** | Low (add more regex) | High (add visitor methods) |
| **Complex SQL** | Cannot handle | Full support |
| **Error Detection** | Runtime failures | Parse-time validation |
| **Type Awareness** | None | Full type context |
| **Performance** | Fast for simple SQL | Fast with caching |

### What Becomes Possible

With AST-based transformation, AProxy could support:

1. ✅ **MATCH AGAINST** → `to_tsvector/to_tsquery`
2. ✅ **Complex CTEs** (WITH clauses)
3. ✅ **Window functions** with MySQL-specific syntax
4. ✅ **JSON path expressions** (MySQL → PostgreSQL JSONB)
5. ✅ **Stored procedures** (MySQL PL → PL/pgSQL)
6. ✅ **Advanced JOIN syntax** (STRAIGHT_JOIN, etc.)
7. ✅ **Type-aware conversions** (TINYINT(1) → BOOLEAN)
8. ✅ **Subquery transformations**
9. ✅ **Operator precedence handling**
10. ✅ **Context-sensitive function rewrites**

## Migration Path

### Phase 1: Parallel Implementation
- Implement AST-based rewriter alongside regex-based
- Add feature flag: `use_ast_rewriter=true`
- Test both paths in parallel

### Phase 2: Gradual Migration
- Start with simple statements (SELECT, INSERT)
- Expand to complex statements (JOIN, subqueries)
- Add MATCH AGAINST support
- Measure performance and correctness

### Phase 3: Deprecation
- Make AST-based rewriter default
- Keep regex-based as fallback for 1-2 releases
- Remove regex-based implementation

### Phase 4: Advanced Features
- Add unsupported feature detection
- Implement stored procedure translation
- Add query optimization hints

## Performance Considerations

**Parsing overhead**: TiDB parser is fast (~1-2ms for typical queries)

**Caching strategy**:
```go
type RewriteCache struct {
    cache sync.Map // map[string]*CachedRewrite
}

type CachedRewrite struct {
    OriginalSQL string
    RewrittenSQL string
    AST ast.Node
}

func (c *RewriteCache) Get(sql string) (string, bool) {
    if v, ok := c.cache.Load(sql); ok {
        return v.(*CachedRewrite).RewrittenSQL, true
    }
    return "", false
}
```

**Expected performance**:
- Simple queries: <1ms overhead
- Complex queries: 2-5ms overhead
- Cached queries: <0.1ms overhead

## Conclusion

The current regex-based approach has served well for basic SQL transformation, but has hit architectural limits. Moving to AST-based transformation is the **correct long-term solution** that will:

1. Enable support for complex MySQL features like MATCH AGAINST
2. Improve correctness and reliability
3. Make the codebase more maintainable
4. Allow AProxy to handle production-grade MySQL applications

**Recommendation**: Begin Phase 1 (parallel implementation) immediately, focusing on:
- Integrating TiDB parser
- Implementing visitor pattern for SELECT/INSERT/UPDATE/DELETE
- Building test suite comparing regex vs AST output
- Measuring performance impact

This is a significant architectural change, but necessary for AProxy to achieve its goal of transparent MySQL → PostgreSQL proxying.

## References

- [TiDB Parser](https://github.com/pingcap/tidb/tree/master/pkg/parser) - Production-ready MySQL parser
- [PostgreSQL Full-Text Search](https://www.postgresql.org/docs/current/textsearch.html) - Target for MATCH AGAINST
- [Visitor Pattern for AST](https://en.wikipedia.org/wiki/Visitor_pattern) - Standard approach for AST transformation
- [MySQL to PostgreSQL Migration](https://www.postgresql.org/docs/current/mysql-to-postgres.html) - SQL dialect differences
