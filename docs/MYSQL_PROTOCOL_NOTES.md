# MySQL Protocol 技术笔记

## MySQL Wire Protocol 基础

### 协议类型对比

MySQL 支持两种主要的结果集传输协议：

| 特性 | Text Protocol | Binary Protocol |
|------|--------------|-----------------|
| 使用场景 | COM_QUERY | COM_STMT_EXECUTE |
| 数据格式 | 字符串 | 二进制 |
| 类型安全 | 弱 | 强 |
| 性能 | 较低 | 较高 |
| NULL 表示 | `0xfb` | Bitmap |
| 数值编码 | ASCII 字符串 | 原生二进制 |

### 关键发现

**重要**: Prepared Statements (COM_STMT_EXECUTE) **必须**使用 Binary Protocol 返回结果，否则客户端会报错！

```go
// 错误示例 - 会导致 "busy buffer" 错误
func HandleStmtExecute(query string, args []interface{}) (*Result, error) {
    rows := executeQuery(query, args)
    // ❌ 使用 Text Protocol
    return buildResult(rows, false)
}

// 正确示例
func HandleStmtExecute(query string, args []interface{}) (*Result, error) {
    rows := executeQuery(query, args)
    // ✅ 使用 Binary Protocol
    return buildResult(rows, true)
}
```

## go-mysql 库使用要点

### BuildSimpleResultset 函数

```go
func BuildSimpleResultset(
    names []string,      // 字段名
    values [][]interface{}, // 数据行
    binary bool,         // 🔑 关键参数：true=Binary Protocol, false=Text Protocol
) (*Resultset, error)
```

**重要注意事项**:

1. **ColumnLength 不会自动设置**
   - `BuildSimpleResultset()` 和 `BuildSimpleTextResultset()` 都不会设置 `Field.ColumnLength`
   - **必须手动设置**，否则客户端可能解析失败

2. **binary 参数的正确使用**
   ```go
   // COM_QUERY - 普通查询
   resultset, _ := mysql.BuildSimpleResultset(names, values, false)

   // COM_STMT_EXECUTE - 预编译语句执行
   resultset, _ := mysql.BuildSimpleResultset(names, values, true)
   ```

### Field 结构体关键字段

```go
type Field struct {
    Name         []byte  // 字段名
    OrgName      []byte  // 原始字段名
    Table        []byte  // 表名（建议设置为空 []byte{}）
    OrgTable     []byte  // 原始表名（建议设置为空 []byte{}）
    Schema       []byte  // 数据库名（建议设置为空 []byte{}）
    Type         byte    // MySQL 类型（如 MYSQL_TYPE_LONGLONG）
    Charset      uint16  // 字符集（63=binary, 33=utf8）
    ColumnLength uint32  // 🔑 显示长度，必须手动设置！
    Flag         uint16  // 标志位（NOT_NULL_FLAG, BINARY_FLAG 等）
    Decimal      uint8   // 小数位数（用于 DECIMAL 类型）
}
```

**ColumnLength 设置指南**:

| PostgreSQL 类型 | MySQL 类型 | ColumnLength 计算方法 |
|----------------|-----------|---------------------|
| INT4, INT8 | LONGLONG | `20` (固定) |
| NUMERIC(p,s) | NEWDECIMAL | `precision + 2` |
| VARCHAR(n) | VAR_STRING | `n` (从 TypeModifier 提取) |
| TEXT | VAR_STRING | `65535` |
| DATE | DATE | `10` ("YYYY-MM-DD") |
| TIMESTAMP | TIMESTAMP | `19` ("YYYY-MM-DD HH:MM:SS") |
| TIME | TIME | `8` ("HH:MM:SS") |

## PostgreSQL 类型元数据解析

### TypeModifier 编码格式

PostgreSQL 使用 TypeModifier 存储类型的额外信息（如精度、长度）。

#### NUMERIC/DECIMAL 类型

**编码格式**: `((precision << 16) | scale) + 4`

```go
// 示例：NUMERIC(10, 2)
// TypeModifier = ((10 << 16) | 2) + 4 = 655366

// 解码方法
typemod := fd.TypeModifier - 4  // 655362
precision := (typemod >> 16) & 0xFFFF  // 10
scale := typemod & 0xFFFF              // 2

// ColumnLength 计算
field.ColumnLength = uint32(precision + 2)  // 12
field.Decimal = uint8(scale)                // 2
```

**为什么是 precision + 2？**
- +1 给小数点 `.`
- +1 给负号 `-`

#### VARCHAR 类型

**编码格式**: `max_length + 4`

```go
// 示例：VARCHAR(255)
// TypeModifier = 255 + 4 = 259

// 解码方法
if fd.TypeModifier > 0 {
    field.ColumnLength = uint32(fd.TypeModifier - 4)  // 255
} else {
    field.ColumnLength = 65535  // 默认最大长度
}
```

### FieldDescription 结构

```go
type FieldDescription struct {
    Name         string  // 字段名
    TableOID     uint32  // 表 OID
    DataTypeOID  uint32  // 🔑 数据类型 OID
    TypeModifier int32   // 🔑 类型修饰符（编码了 precision/scale/length）
    // ...
}
```

**常用 PostgreSQL OID**:

| OID | 类型 | 说明 |
|-----|------|-----|
| 23 | INT4 | 4 字节整数 |
| 20 | INT8 | 8 字节整数 |
| 1700 | NUMERIC | 任意精度数值 |
| 25 | TEXT | 变长文本 |
| 1043 | VARCHAR | 变长字符串 |
| 1082 | DATE | 日期 |
| 1114 | TIMESTAMP | 时间戳（无时区） |
| 1184 | TIMESTAMPTZ | 时间戳（带时区） |
| 1083 | TIME | 时间 |

## 调试技巧

### 1. Field.Dump() 输出分析

在 go-mysql 中添加 DEBUG 日志：

```go
// vendor-go-mysql/mysql/field.go
func (f *Field) Dump() []byte {
    data := /* ... 编码逻辑 ... */

    // DEBUG 输出
    fmt.Printf("DEBUG Field.Dump(): Name=%s, Type=%d, Charset=%d, "+
        "ColumnLength=%d, Flag=%d, Decimal=%d, DumpLen=%d, DumpHex=%X\n",
        string(f.Name), f.Type, f.Charset, f.ColumnLength,
        f.Flag, f.Decimal, len(data), data)

    return data
}
```

**示例输出**:
```
DEBUG Field.Dump(): Name=id, Type=8, Charset=63, ColumnLength=20, Flag=129, Decimal=0
DEBUG Field.Dump(): Name=price, Type=246, Charset=63, ColumnLength=12, Flag=129, Decimal=2
```

**分析要点**:
- `Type=8`: MYSQL_TYPE_LONGLONG
- `Type=246`: MYSQL_TYPE_NEWDECIMAL
- `Charset=63`: Binary charset
- `ColumnLength=0`: ⚠️ 未设置，需要修复！

### 2. writeResultset 跟踪

```go
// vendor-go-mysql/server/resp.go
func (c *Conn) writeResultset(r *Resultset) error {
    fmt.Printf("DEBUG writeResultset: Fields=%d, RowDatas=%d\n",
        len(r.Fields), len(r.RowDatas))
    // ...
}
```

### 3. 创建最小复现测试

```go
// /tmp/test_prepared_multifield.go
package main

import (
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

func main() {
    db, _ := sql.Open("mysql", "root@tcp(127.0.0.1:3306)/test")
    defer db.Close()

    // 准备语句
    stmt, err := db.Prepare("SELECT id, quantity FROM test_table WHERE id = ?")
    if err != nil {
        panic(err)
    }
    defer stmt.Close()

    // 执行查询
    rows, err := stmt.Query(1)
    if err != nil {
        panic(err)  // 这里会触发 "busy buffer" 错误
    }
    defer rows.Close()

    // 读取结果
    for rows.Next() {
        var id, quantity int
        rows.Scan(&id, &quantity)
        println("SUCCESS:", id, quantity)
    }
}
```

## 常见错误与解决方案

### 错误 1: "busy buffer"

**症状**:
- Prepared Statement 执行时客户端报错
- 单字段查询正常，多字段查询失败

**根因**:
- 使用了 Text Protocol 而非 Binary Protocol
- ColumnLength 未设置或设置错误

**解决**:
```go
// 确保 Prepared Statement 使用 Binary Protocol
func (h *Handler) HandleStmtExecute(ctx, query, args) (*Result, error) {
    rows := h.executeQuery(query, args)
    return h.buildResult(rows, true)  // ✅ binary=true
}
```

### 错误 2: DECIMAL 显示异常

**症状**:
- DECIMAL 字段显示为很大的数字
- 精度丢失

**根因**:
- ColumnLength 从 TypeModifier 计算错误
- 直接使用 `TypeModifier - 4` 而未提取 precision

**解决**:
```go
case 1700: // NUMERIC/DECIMAL
    if fd.TypeModifier > 0 {
        typemod := fd.TypeModifier - 4
        precision := (typemod >> 16) & 0xFFFF  // ✅ 提取高 16 位
        scale := typemod & 0xFFFF              // ✅ 提取低 16 位
        field.ColumnLength = uint32(precision + 2)
        field.Decimal = uint8(scale)
    }
```

### 错误 3: VARCHAR 长度限制失效

**症状**:
- VARCHAR(50) 可以插入超长数据
- 客户端显示字段长度不正确

**根因**:
- ColumnLength 未从 TypeModifier 提取

**解决**:
```go
case 1043: // VARCHAR
    if fd.TypeModifier > 0 {
        field.ColumnLength = uint32(fd.TypeModifier - 4)  // ✅ 提取长度
    } else {
        field.ColumnLength = 65535  // 默认值
    }
```

## 最佳实践

### 1. 总是手动设置 ColumnLength

```go
// ❌ 错误：依赖默认值
resultset, _ := mysql.BuildSimpleResultset(names, values, binary)
return &mysql.Result{Resultset: resultset}

// ✅ 正确：手动设置每个字段
resultset, _ := mysql.BuildSimpleResultset(names, values, binary)
for i, field := range resultset.Fields {
    fd := fieldDescs[i]
    // 根据 PostgreSQL 类型设置正确的 ColumnLength
    field.ColumnLength = calculateColumnLength(fd)
}
return &mysql.Result{Resultset: resultset}
```

### 2. 区分不同的命令类型

```go
func (h *Handler) HandleQuery(query string) (*Result, error) {
    // 普通查询使用 Text Protocol
    return h.buildResult(rows, false)
}

func (h *Handler) HandleStmtExecute(query string, args []interface{}) (*Result, error) {
    // Prepared Statement 使用 Binary Protocol
    return h.buildResult(rows, true)
}

func (h *Handler) handleShowCommand(query string) (*Result, error) {
    // SHOW 命令使用 Text Protocol
    return h.buildResult(rows, false)
}
```

### 3. 使用类型安全的映射表

```go
var pgToMySQLColumnLength = map[uint32]func(int32) uint32{
    23: func(mod int32) uint32 { return 20 },  // INT4
    20: func(mod int32) uint32 { return 20 },  // INT8
    1700: func(mod int32) uint32 {  // NUMERIC
        if mod > 0 {
            typemod := mod - 4
            precision := (typemod >> 16) & 0xFFFF
            return uint32(precision + 2)
        }
        return 12
    },
    1043: func(mod int32) uint32 {  // VARCHAR
        if mod > 0 {
            return uint32(mod - 4)
        }
        return 65535
    },
}
```

## 参考资料

### MySQL 官方文档
- [Client/Server Protocol](https://dev.mysql.com/doc/dev/mysql-server/latest/PAGE_PROTOCOL.html)
- [Text Protocol](https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_com_query_response_text_resultset.html)
- [Binary Protocol](https://dev.mysql.com/doc/dev/mysql-server/latest/page_protocol_binary_resultset.html)

### PostgreSQL 官方文档
- [Frontend/Backend Protocol](https://www.postgresql.org/docs/current/protocol.html)
- [Data Type OIDs](https://github.com/postgres/postgres/blob/master/src/include/catalog/pg_type.dat)

### 相关库
- [go-mysql](https://github.com/go-mysql-org/go-mysql) - MySQL 协议实现
- [pgx](https://github.com/jackc/pgx) - PostgreSQL 驱动

## 调试检查清单

在实现 MySQL 协议代理时，使用此检查清单确保正确性：

- [ ] Prepared Statements 使用 Binary Protocol (`binary=true`)
- [ ] 普通查询使用 Text Protocol (`binary=false`)
- [ ] 所有 Field 都设置了正确的 ColumnLength
- [ ] DECIMAL 类型正确提取了 precision 和 scale
- [ ] VARCHAR 类型正确提取了最大长度
- [ ] Schema/Table/OrgTable 字段设置为空 `[]byte{}`（除非有特殊需求）
- [ ] Charset 根据类型正确设置（63=binary, 33=utf8）
- [ ] Flag 字段正确设置（NOT_NULL_FLAG, BINARY_FLAG 等）
- [ ] 添加了充分的测试用例覆盖单字段和多字段场景
- [ ] 测试了不同数据类型的组合
