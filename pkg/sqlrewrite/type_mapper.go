package sqlrewrite

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/types"
)

// TypeMapper handles MySQL to PostgreSQL type conversions
// using TiDB Parser's type system
type TypeMapper struct {
	// Future: could add custom type mappings
}

// NewTypeMapper creates a new type mapper instance
func NewTypeMapper() *TypeMapper {
	return &TypeMapper{}
}

// MySQLToPostgreSQL converts a MySQL FieldType to PostgreSQL type string
// Follows TiDB Parser's type constants from mysql.Type* constants
func (m *TypeMapper) MySQLToPostgreSQL(tp *types.FieldType) string {
	if tp == nil {
		return "TEXT" // Safe fallback
	}

	mysqlType := tp.GetType()
	isUnsigned := mysql.HasUnsignedFlag(tp.GetFlag())
	flen := tp.GetFlen()
	decimal := tp.GetDecimal()

	switch mysqlType {
	// Integer types
	case mysql.TypeTiny:
		// TINYINT -> SMALLINT (PostgreSQL has no TINYINT)
		return "SMALLINT"

	case mysql.TypeShort:
		// SMALLINT -> SMALLINT
		return "SMALLINT"

	case mysql.TypeLong, mysql.TypeInt24:
		// INT, MEDIUMINT -> INTEGER or BIGINT (if unsigned)
		if isUnsigned {
			// Unsigned INT needs BIGINT to avoid overflow
			return "BIGINT"
		}
		return "INTEGER"

	case mysql.TypeLonglong:
		// BIGINT -> BIGINT
		if isUnsigned {
			// PostgreSQL doesn't have unsigned, use NUMERIC(20,0) for safety
			return "NUMERIC(20,0)"
		}
		return "BIGINT"

	// Floating point types
	case mysql.TypeFloat:
		// FLOAT -> REAL
		return "REAL"

	case mysql.TypeDouble:
		// DOUBLE -> DOUBLE PRECISION
		return "DOUBLE PRECISION"

	case mysql.TypeNewDecimal:
		// DECIMAL(M,D) -> NUMERIC(M,D)
		if flen > 0 {
			if decimal > 0 {
				return fmt.Sprintf("NUMERIC(%d,%d)", flen, decimal)
			}
			return fmt.Sprintf("NUMERIC(%d)", flen)
		}
		return "NUMERIC"

	// Date and time types
	case mysql.TypeDate:
		// DATE -> DATE
		return "DATE"

	case mysql.TypeDatetime, mysql.TypeTimestamp:
		// DATETIME, TIMESTAMP -> TIMESTAMP
		// Note: MySQL TIMESTAMP has different timezone behavior than PostgreSQL
		if flen > 0 {
			// Support microsecond precision
			return fmt.Sprintf("TIMESTAMP(%d)", flen)
		}
		return "TIMESTAMP"

	case mysql.TypeDuration:
		// TIME (TypeDuration) -> TIME
		// Note: TypeTime was renamed to TypeDuration in TiDB parser
		if flen > 0 {
			return fmt.Sprintf("TIME(%d)", flen)
		}
		return "TIME"

	case mysql.TypeYear:
		// YEAR -> SMALLINT (PostgreSQL has no YEAR type)
		return "SMALLINT"

	// String types
	case mysql.TypeVarchar, mysql.TypeVarString:
		// VARCHAR(N) -> VARCHAR(N)
		if flen > 0 && flen <= 65535 {
			return fmt.Sprintf("VARCHAR(%d)", flen)
		}
		return "TEXT"

	case mysql.TypeString:
		// CHAR(N) -> CHAR(N)
		if flen > 0 && flen <= 255 {
			return fmt.Sprintf("CHAR(%d)", flen)
		}
		return "TEXT"

	case mysql.TypeTinyBlob:
		// TINYTEXT, TINYBLOB -> TEXT or BYTEA
		if mysql.HasBinaryFlag(tp.GetFlag()) {
			return "BYTEA"
		}
		return "TEXT"

	case mysql.TypeBlob:
		// TEXT, BLOB -> TEXT or BYTEA
		if mysql.HasBinaryFlag(tp.GetFlag()) {
			return "BYTEA"
		}
		return "TEXT"

	case mysql.TypeMediumBlob:
		// MEDIUMTEXT, MEDIUMBLOB -> TEXT or BYTEA
		if mysql.HasBinaryFlag(tp.GetFlag()) {
			return "BYTEA"
		}
		return "TEXT"

	case mysql.TypeLongBlob:
		// LONGTEXT, LONGBLOB -> TEXT or BYTEA
		if mysql.HasBinaryFlag(tp.GetFlag()) {
			return "BYTEA"
		}
		return "TEXT"

	// JSON type
	case mysql.TypeJSON:
		// JSON -> JSONB (more efficient in PostgreSQL)
		return "JSONB"

	// Enum and Set (limited support)
	case mysql.TypeEnum:
		// ENUM -> TEXT with CHECK constraint (simplified)
		// Full support would require extracting enum values
		return "TEXT"

	case mysql.TypeSet:
		// SET -> TEXT[] (array type)
		return "TEXT[]"

	// Binary types
	case mysql.TypeBit:
		// BIT(N) -> BIT(N) or VARBIT(N)
		if flen > 0 {
			return fmt.Sprintf("BIT(%d)", flen)
		}
		return "BIT"

	// Geometry types (basic support)
	case mysql.TypeGeometry:
		// Requires PostGIS extension
		return "GEOMETRY"

	// Null type
	case mysql.TypeNull:
		return "TEXT"

	// Default fallback
	default:
		return "TEXT"
	}
}

// MySQLToPostgreSQLString converts a MySQL type string to PostgreSQL type string
// This is a simpler version that works with raw type strings
func (m *TypeMapper) MySQLToPostgreSQLString(mysqlType string) string {
	upper := strings.ToUpper(strings.TrimSpace(mysqlType))

	// Extract base type and length/precision
	baseType := upper
	if idx := strings.Index(upper, "("); idx != -1 {
		baseType = strings.TrimSpace(upper[:idx])
	}

	// Simple string-based mapping
	switch baseType {
	case "TINYINT":
		return strings.Replace(upper, "TINYINT", "SMALLINT", 1)
	case "INT", "INTEGER", "MEDIUMINT":
		// Check for UNSIGNED
		if strings.Contains(upper, "UNSIGNED") {
			return strings.Replace(
				strings.Replace(upper, "UNSIGNED", "", 1),
				baseType, "BIGINT", 1,
			)
		}
		return strings.Replace(upper, baseType, "INTEGER", 1)
	case "BIGINT":
		if strings.Contains(upper, "UNSIGNED") {
			return "NUMERIC(20,0)"
		}
		return upper
	case "FLOAT":
		return strings.Replace(upper, "FLOAT", "REAL", 1)
	case "DOUBLE", "DOUBLE PRECISION":
		return "DOUBLE PRECISION"
	case "DECIMAL", "NUMERIC":
		return strings.Replace(upper, "DECIMAL", "NUMERIC", 1)
	case "DATETIME", "TIMESTAMP":
		return strings.Replace(
			strings.Replace(upper, "DATETIME", "TIMESTAMP", 1),
			"TIMESTAMP", "TIMESTAMP", 1,
		)
	case "YEAR":
		return "SMALLINT"
	case "TINYTEXT", "TINYBLOB":
		if strings.Contains(baseType, "BLOB") {
			return "BYTEA"
		}
		return "TEXT"
	case "TEXT", "MEDIUMTEXT", "LONGTEXT":
		return "TEXT"
	case "BLOB", "MEDIUMBLOB", "LONGBLOB":
		return "BYTEA"
	case "JSON":
		return "JSONB"
	case "ENUM":
		return "TEXT"
	case "SET":
		return "TEXT[]"
	default:
		return upper
	}
}

// GetPostgreSQLDefaultValue converts MySQL default values to PostgreSQL format
func (m *TypeMapper) GetPostgreSQLDefaultValue(mysqlDefault string, pgType string) string {
	if mysqlDefault == "" {
		return ""
	}

	upper := strings.ToUpper(mysqlDefault)

	// Function call conversions
	switch upper {
	case "NOW()", "CURRENT_TIMESTAMP()", "CURRENT_TIMESTAMP":
		return "CURRENT_TIMESTAMP"
	case "CURDATE()", "CURRENT_DATE()", "CURRENT_DATE":
		return "CURRENT_DATE"
	case "CURTIME()", "CURRENT_TIME()", "CURRENT_TIME":
		return "CURRENT_TIME"
	}

	// Boolean conversions for TINYINT(1) -> BOOLEAN mapping
	if strings.HasPrefix(pgType, "BOOLEAN") {
		if mysqlDefault == "0" || upper == "FALSE" {
			return "FALSE"
		}
		if mysqlDefault == "1" || upper == "TRUE" {
			return "TRUE"
		}
	}

	// Return as-is for other cases
	return mysqlDefault
}

// IsBooleanType checks if a MySQL TINYINT(1) should be treated as BOOLEAN
func (m *TypeMapper) IsBooleanType(tp *types.FieldType) bool {
	if tp == nil {
		return false
	}

	// MySQL convention: TINYINT(1) is often used for boolean
	return tp.GetType() == mysql.TypeTiny && tp.GetFlen() == 1
}

// MySQLToPostgreSQLBoolean converts TINYINT(1) to BOOLEAN if appropriate
func (m *TypeMapper) MySQLToPostgreSQLBoolean(tp *types.FieldType) string {
	if m.IsBooleanType(tp) {
		return "BOOLEAN"
	}
	return m.MySQLToPostgreSQL(tp)
}
