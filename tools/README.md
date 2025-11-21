# Development and Debugging Tools

This directory contains utility tools for development, debugging, and verification. These are not automated tests but manual tools for developers.

## Tools

### debug_field_dump.go

**Purpose**: Debug MySQL protocol field packets and row data serialization

**Usage**:
```bash
go run tools/debug_field_dump.go
```

**What it does**:
- Creates MySQL Resultset with sample fields (id, name, price)
- Dumps field definitions in hexadecimal format
- Shows RowData serialization
- Useful for debugging MySQL protocol implementation issues

**When to use**:
- Debugging field packet encoding issues
- Verifying MySQL protocol compatibility
- Understanding field type flags and charsets

**Output example**:
```
=== Field Definitions ===
Field 0: id
  Type: 8 (MYSQL_TYPE_LONGLONG)
  Charset: 63
  ColumnLength: 20
  Flag: 128 (BINARY_FLAG)

=== Field Packets (Hex Dump) ===
Field 0 packet (len=26): 03 64 65 66 ...
```

---

### verify_pg_text_format.go

**Purpose**: Verify PostgreSQL Text Format configuration for pgx driver

**Usage**:
```bash
# Make sure PostgreSQL is running and configured
go run tools/verify_pg_text_format.go
```

**What it does**:
- Connects to PostgreSQL with Simple Query Protocol
- Creates a test table and inserts data
- Verifies field format is Text (Format == 0) not Binary (Format == 1)
- Tests both direct connection and connection pool modes

**When to use**:
- Before deploying AProxy to ensure PostgreSQL is configured correctly
- Debugging type conversion issues (binary vs text format)
- Verifying pgx driver configuration

**Requirements**:
- PostgreSQL server running on `localhost:5432`
- Database `aproxy_test` exists
- User `aproxy_user` with password `aproxy_pass` (or modify connection string)

**Output example**:
```
=== Verify Text Format Configuration ===

Test 1: Direct Simple Query Protocol
DefaultQueryExecMode set to: 0

Field descriptions:
  [0] Name: id, DataTypeOID: 23, Format: 0 (Text Format ✓)
  [1] Name: name, DataTypeOID: 1043, Format: 0 (Text Format ✓)
  [2] Name: price, DataTypeOID: 1700, Format: 0 (Text Format ✓)
```

**Note**: This tool connects directly to PostgreSQL, not through AProxy. It's for verifying the backend database configuration.

---

### debug_resultset.go

**Purpose**: Test the BuildSimpleTextResultset function for MySQL protocol resultset generation

**Usage**:
```bash
go run tools/debug_resultset.go
```

**What it does**:
- Tests BuildSimpleTextResultset with sample data (id, name, price)
- Dumps field definitions and properties
- Shows hexadecimal dump of RowData serialization
- Verifies resultset building logic

**When to use**:
- Debugging BuildSimpleTextResultset function implementation
- Verifying MySQL protocol resultset structure
- Understanding field encoding and row data serialization
- Testing changes to resultset building code

**Output example**:
```
=== Testing BuildSimpleTextResultset ===

Fields:
Field 0: id
  Type: 8 (MYSQL_TYPE_LONGLONG)
  Flag: 128
  Charset: 63

Field 1: name
  Type: 253 (MYSQL_TYPE_VARCHAR)
  Flag: 0
  Charset: 33

=== RowData Hex Dump ===
Row 0 (len=28): 01 31 09 50 72 6f 64 75 63 74 20 31 ...
```

**Note**: This tool is for testing the internal resultset building logic, not for testing AProxy's proxy functionality.

---

## Adding New Tools

When adding new tools to this directory:

1. Use descriptive filenames (e.g., `debug_xxx.go`, `verify_xxx.go`)
2. Add documentation to this README
3. Include usage examples and sample output
4. Add comment headers to explain the tool's purpose
5. Use `package main` and provide a `main()` function

## Why Not Test Cases?

These tools are kept separate from automated tests because:

- **debug_field_dump.go**: Outputs hex dumps that require manual inspection, not suitable for automated assertions
- **verify_pg_text_format.go**: Tests PostgreSQL configuration directly, not AProxy functionality
- **debug_resultset.go**: Outputs hex dumps for manual verification of resultset building logic

For automated testing of AProxy functionality, see `test/integration/` directory.
