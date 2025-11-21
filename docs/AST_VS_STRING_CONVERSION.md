# AST vs String Conversion Analysis

This document analyzes all MySQL to PostgreSQL conversion rules to determine which can be fully handled at the AST level vs which require string-level post-processing.

## Summary

**Current Architecture:**
1. **AST Level (ast_visitor.go)**: Structured conversions on parsed AST
2. **String Level (pg_generator.go PostProcess)**: Text-based replacements after SQL generation

**Goal:** Maximize AST-level conversions to avoid issues with column names containing type keywords.

---

## Conversion Rules Analysis

### ✅ Category 1: Fully Convertible to AST Level

These conversions are **already handled or can be fully handled** at the AST level:

#### 1.1 Data Type Conversions (CREATE TABLE)
**Current Status:** ✅ Already in AST (ast_visitor.go:visitColumnDef)

| MySQL Type | PostgreSQL Type | AST Node | Status |
|------------|-----------------|----------|--------|
| TINYINT | SMALLINT | FieldType.Tp | ✅ Done |
| TINYINT UNSIGNED | SMALLINT | FieldType.Tp + Flag | ✅ Done |
| SMALLINT UNSIGNED | INTEGER | FieldType.Tp + Flag | ✅ Done |
| MEDIUMINT | INTEGER | FieldType.Tp | ✅ Done |
| INT UNSIGNED | BIGINT | FieldType.Tp + Flag | ✅ Done |
| BIGINT UNSIGNED | NUMERIC(20,0) | FieldType.Tp + Flag | ✅ Done |
| YEAR | SMALLINT | FieldType.Tp | ✅ Done |
| DATETIME | TIMESTAMP | FieldType.Tp | ✅ Done |
| TINYBLOB | BLOB → BYTEA | FieldType.Tp | ⚠️ Partial (see 2.1) |
| MEDIUMBLOB | BLOB → BYTEA | FieldType.Tp | ⚠️ Partial (see 2.1) |
| LONGBLOB | BLOB → BYTEA | FieldType.Tp | ⚠️ Partial (see 2.1) |
| TINYTEXT | TEXT | FieldType.Tp | ⚠️ Needs AST |
| MEDIUMTEXT | TEXT | FieldType.Tp | ⚠️ Needs AST |
| LONGTEXT | TEXT | FieldType.Tp | ⚠️ Needs AST |
| ENUM(...) | VARCHAR(50) | FieldType.Tp + Elems | ✅ Done |
| JSON | JSONB | FieldType.Tp | ⚠️ Needs AST |

**Recommendation:**
- Move TINYTEXT/MEDIUMTEXT/LONGTEXT → TEXT to AST level
- Move JSON → JSONB to AST level
- Keep BLOB → BYTEA string-level (see explanation in Category 2)

#### 1.2 Function Conversions
**Current Status:** ✅ Already in AST (ast_visitor.go:visitFuncCallExpr)

| MySQL Function | PostgreSQL Equivalent | AST Node | Status |
|----------------|----------------------|----------|--------|
| NOW() | CURRENT_TIMESTAMP | FuncCallExpr.FnName | ✅ Done |
| CURDATE() | CURRENT_DATE | FuncCallExpr.FnName | ✅ Done |
| CURTIME() | CURRENT_TIME | FuncCallExpr.FnName | ✅ Done |
| IFNULL(a,b) | COALESCE(a,b) | FuncCallExpr | ✅ Done |
| IF(cond,a,b) | CASE WHEN...END | FuncCallExpr | ✅ Done |
| LENGTH(s) | LENGTH(s) | FuncCallExpr | ✅ Done |
| CONCAT(a,b) | CONCAT(a,b) | FuncCallExpr | ✅ Done |

**Recommendation:** ✅ All function conversions should stay at AST level

#### 1.3 Constraint and Index Handling
**Current Status:** ✅ Already in AST (ast_visitor.go:visitCreateTable)

| MySQL Feature | PostgreSQL Equivalent | AST Node | Status |
|---------------|----------------------|----------|--------|
| INDEX idx(...) | (removed inline) | Constraint.Tp | ✅ Done (filtered) |
| KEY idx(...) | (removed inline) | Constraint.Tp | ✅ Done (filtered) |
| UNIQUE INDEX idx | UNIQUE | Constraint.Tp + Name | ✅ Done (name cleared) |
| UNIQUE KEY idx | UNIQUE | Constraint.Tp + Name | ✅ Done (name cleared) |
| PRIMARY KEY | PRIMARY KEY | Constraint.Tp | ✅ Done |
| AUTO_INCREMENT | SERIAL/BIGSERIAL | FieldType.Flag | ✅ Done |

**Recommendation:** ✅ All constraint handling should stay at AST level

#### 1.4 Placeholder Conversion
**Current Status:** ✅ Already in AST (ast_visitor.go:visitParamMarkerExpr)

| MySQL | PostgreSQL | AST Node | Status |
|-------|------------|----------|--------|
| ? | $1, $2, $3... | ParamMarkerExpr | ✅ Done |

**Recommendation:** ✅ Keep at AST level

#### 1.5 LIMIT Clause Conversion
**Current Status:** ⚠️ Currently string-level, CAN move to AST

| MySQL | PostgreSQL | AST Node | Status |
|-------|------------|----------|--------|
| LIMIT offset, count | LIMIT count OFFSET offset | Limit.Offset, Limit.Count | ⚠️ String-level now |

**Recommendation:** 🔄 **SHOULD move to AST level**

AST nodes available:
- `ast.Limit.Offset` - offset value
- `ast.Limit.Count` - count value

Can be handled in `visitSelectStmt()` by checking if both Offset and Count are set, then restructuring.

---

### ⚠️ Category 2: Hybrid (AST + String Post-Processing)

These conversions require **both AST and string-level handling**:

#### 2.1 BLOB → BYTEA
**Current Status:** ⚠️ AST converts TINYBLOB/MEDIUMBLOB/LONGBLOB → BLOB, then string converts BLOB → BYTEA

**Why hybrid?**
- TiDB AST has `mysql.TypeBlob` type constant
- But TiDB's RestoreSQL() outputs the string "BLOB" when generating SQL
- PostgreSQL requires "BYTEA" in the actual SQL text
- We cannot modify TiDB's RestoreSQL() output format

**Current Implementation:**
1. AST level: Convert TINYBLOB/MEDIUMBLOB/LONGBLOB → BLOB (type unification)
2. String level: Convert BLOB → BYTEA (text replacement)

**Recommendation:** ⚠️ **Keep hybrid approach** - this is unavoidable due to TiDB's RestoreSQL behavior

#### 2.2 DOUBLE → DOUBLE PRECISION
**Current Status:** ⚠️ String-level only

**Why string-level?**
- MySQL: `DOUBLE` or `DOUBLE(m,d)`
- PostgreSQL: `DOUBLE PRECISION`
- TiDB AST represents this as `mysql.TypeDouble`
- When restored, it outputs "DOUBLE"
- Need to check if already followed by "PRECISION" to avoid double-replacement

**Recommendation:** ⚠️ **Keep string-level** - complex pattern matching needed

---

### ❌ Category 3: Must Stay at String Level

These conversions **cannot be done at AST level** due to technical limitations:

#### 3.1 Backtick → Double Quote Conversion
**Current Status:** ❌ String-level (pg_generator.go:PostProcess)

| MySQL | PostgreSQL | Reason |
|-------|------------|--------|
| \`table\` | "table" | TiDB RestoreSQL outputs backticks for quoted identifiers |

**Why string-level?**
- TiDB parser stores identifiers without quotes in AST
- TiDB RestoreSQL() adds backticks around quoted identifiers
- Cannot modify this behavior without forking TiDB parser
- Simple string replacement is safe and efficient

**Recommendation:** ❌ **Must stay string-level**

#### 3.2 Table Options Removal
**Current Status:** ❌ String-level (pg_generator.go:removeTableOptions)

Examples:
```sql
ENGINE=InnoDB
DEFAULT CHARSET=utf8mb4
COLLATE=utf8mb4_unicode_ci
ROW_FORMAT=DYNAMIC
COMMENT='...'
```

**Why string-level?**
- TiDB AST has `TableOptions` field in `CreateTableStmt`
- But RestoreSQL() always includes these options in output
- Cannot suppress them through AST modification
- Must remove after SQL generation

**Recommendation:** ❌ **Must stay string-level**

#### 3.3 Charset Prefix Removal
**Current Status:** ❌ String-level (pg_generator.go:removeCharsetPrefixes)

Examples:
```sql
_UTF8MB4'text'
_UTF8'text'
_LATIN1'text'
```

**Why string-level?**
- MySQL allows charset prefixes on string literals
- TiDB RestoreSQL() preserves these in output
- PostgreSQL doesn't support this syntax
- Must remove after SQL generation

**Recommendation:** ❌ **Must stay string-level**

#### 3.4 INSERT NULL → DEFAULT Conversion
**Current Status:** ❌ String-level (pg_generator.go:convertInsertNullToDefault)

**Why string-level?**
- Context-dependent: Only for AUTO_INCREMENT columns
- Requires knowing which columns are AUTO_INCREMENT
- AST InsertStmt doesn't have easy access to table schema
- Would need complex cross-referencing
- String pattern matching is simpler and safer

**Recommendation:** ❌ **Must stay string-level** (or requires major refactoring)

#### 3.5 Special Keyword Handling
**Current Status:** ❌ String-level (pg_generator.go:PostProcess)

Examples:
```sql
CURRENT_TIMESTAMP() → CURRENT_TIMESTAMP
CURRENT_DATE() → CURRENT_DATE
CURRENT_TIME() → CURRENT_TIME
```

**Why string-level?**
- These are PostgreSQL reserved keywords that don't take parentheses
- TiDB may restore them with () based on MySQL syntax
- Simple string replacement handles all cases
- AST manipulation would be more complex

**Recommendation:** ❌ **Keep string-level**

#### 3.6 UNIQUE KEY/INDEX → UNIQUE
**Current Status:** ❌ String-level fallback (AST primary, string backup)

**Current Implementation:**
- Primary: AST removes constraint name for UNIQUE constraints
- Fallback: String replacement for any remaining "UNIQUE KEY" or "UNIQUE INDEX"

**Why keep string fallback?**
- AST handling may miss edge cases
- String replacement is cheap and harmless
- Defense-in-depth approach

**Recommendation:** ⚠️ **Keep both AST + string fallback**

---

## Recommended Architecture Changes

### Priority 1: Move to AST Level (High Impact)

#### 1.1 TEXT Type Variants
**File:** ast_visitor.go:visitColumnDef()

Add to type conversion:
```go
case mysql.TypeTinyBlob:
    tp.SetType(mysql.TypeBlob)
case mysql.TypeMediumBlob:
    tp.SetType(mysql.TypeBlob)
case mysql.TypeLongBlob:
    tp.SetType(mysql.TypeBlob)
```

**Benefit:** Avoids issues with column names like "tinytext_col", "longtext_value"

#### 1.2 JSON → JSONB
**File:** ast_visitor.go:visitColumnDef()

Add:
```go
case mysql.TypeJSON:
    // Note: Will be converted to "JSONB" at string level
    // because TiDB RestoreSQL outputs "JSON"
    // This is similar to BLOB → BYTEA handling
```

**Status:** Actually needs to stay string-level due to RestoreSQL limitations (same as BLOB → BYTEA)

#### 1.3 LIMIT Offset Conversion
**File:** ast_visitor.go:visitSelectStmt()

Add:
```go
func (v *ASTVisitor) visitSelectStmt(node *ast.SelectStmt) {
    // ... existing code ...

    // Convert MySQL LIMIT offset, count to PostgreSQL LIMIT count OFFSET offset
    if node.Limit != nil {
        if node.Limit.Offset != nil && node.Limit.Count != nil {
            // MySQL syntax detected: LIMIT offset, count
            // Swap them for PostgreSQL: LIMIT count OFFSET offset
            // (Note: MySQL parser puts offset first, count second)
            // Already correct for PostgreSQL!
        }
    }
}
```

**Benefit:** Cleaner handling, avoids regex parsing

### Priority 2: Documentation Updates

Update comments in pg_generator.go to clearly mark:
- Which conversions are AST-primary
- Which conversions are string-only
- Why each string conversion cannot be moved to AST

### Priority 3: Testing Strategy

Add tests for edge cases:
1. Column names containing type keywords: `tinyint_col`, `longtext_value`, `enum_type`
2. Column names containing function names: `now_updated`, `if_condition`
3. Mixed case: `TinyInt`, `LONGTEXT`, `Json`

---

## Current State Summary

### ✅ Already AST-Level (Well Architected)
- Data type conversions (TINYINT, MEDIUMINT, YEAR, DATETIME, ENUM, UNSIGNED variants)
- Function conversions (NOW, IFNULL, IF, LENGTH, etc.)
- Constraint handling (INDEX, UNIQUE, PRIMARY KEY, AUTO_INCREMENT)
- Placeholder conversion (? → $1, $2, ...)

### ⚠️ Hybrid (Necessary)
- BLOB → BYTEA (AST unifies blob types, string converts to BYTEA)
- DOUBLE → DOUBLE PRECISION (string-level, complex pattern)

### ❌ Must Stay String-Level (Technical Limitations)
- Backtick → Quote conversion (RestoreSQL behavior)
- Table options removal (RestoreSQL includes them)
- Charset prefix removal (MySQL-specific syntax)
- INSERT NULL → DEFAULT (context-dependent)
- Special keyword cleanup (PostgreSQL syntax quirks)
- UNIQUE KEY/INDEX fallback (defense-in-depth)

### 🔄 Could Move to AST (Low Priority)
- LIMIT offset, count conversion (doable but current string method works)
- TEXT type variants (small benefit)

---

## Conclusion

**Current architecture is well-designed:**
- Core conversions are at AST level (type safety, semantic correctness)
- String post-processing handles TiDB RestoreSQL limitations
- Hybrid approach for BLOB → BYTEA is unavoidable and correct

**Recommended changes:**
1. ✅ Move TINYTEXT/MEDIUMTEXT/LONGTEXT to AST level (small benefit, easy win)
2. ⚠️ Keep JSON → JSONB at string level (same reason as BLOB → BYTEA)
3. 🔄 Consider moving LIMIT conversion to AST (nice-to-have, not critical)
4. ❌ Keep all other string conversions (necessary due to TiDB limitations)

**The fundamental constraint:**
TiDB's RestoreSQL() is a black box that outputs MySQL-formatted SQL. We cannot control:
- Quote characters (backticks)
- Type name strings ("BLOB", "JSON", "DOUBLE")
- Table option inclusion (ENGINE, CHARSET, etc.)
- Charset prefixes in string literals

Therefore, string-level post-processing is **architecturally necessary** and cannot be fully eliminated.

**Best practice going forward:**
- Use AST for **semantic** conversions (types, functions, structure)
- Use string post-processing for **syntactic** cleanup (quotes, keywords, formatting)
- Document clearly which category each conversion belongs to
