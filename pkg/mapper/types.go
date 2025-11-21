package mapper

import (
	"fmt"
	"time"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	MYSQL_TYPE_DECIMAL     = 0x00
	MYSQL_TYPE_TINY        = 0x01
	MYSQL_TYPE_SHORT       = 0x02
	MYSQL_TYPE_LONG        = 0x03
	MYSQL_TYPE_FLOAT       = 0x04
	MYSQL_TYPE_DOUBLE      = 0x05
	MYSQL_TYPE_NULL        = 0x06
	MYSQL_TYPE_TIMESTAMP   = 0x07
	MYSQL_TYPE_LONGLONG    = 0x08
	MYSQL_TYPE_INT24       = 0x09
	MYSQL_TYPE_DATE        = 0x0a
	MYSQL_TYPE_TIME        = 0x0b
	MYSQL_TYPE_DATETIME    = 0x0c
	MYSQL_TYPE_YEAR        = 0x0d
	MYSQL_TYPE_NEWDATE     = 0x0e
	MYSQL_TYPE_VARCHAR     = 0x0f
	MYSQL_TYPE_BIT         = 0x10
	MYSQL_TYPE_JSON        = 0xf5
	MYSQL_TYPE_NEWDECIMAL  = 0xf6
	MYSQL_TYPE_ENUM        = 0xf7
	MYSQL_TYPE_SET         = 0xf8
	MYSQL_TYPE_TINY_BLOB   = 0xf9
	MYSQL_TYPE_MEDIUM_BLOB = 0xfa
	MYSQL_TYPE_LONG_BLOB   = 0xfb
	MYSQL_TYPE_BLOB        = 0xfc
	MYSQL_TYPE_VAR_STRING  = 0xfd
	MYSQL_TYPE_STRING      = 0xfe
	MYSQL_TYPE_GEOMETRY    = 0xff
)

type TypeMapper struct{}

func NewTypeMapper() *TypeMapper {
	return &TypeMapper{}
}

func (tm *TypeMapper) PostgreSQLToMySQL(pgType uint32) byte {
	switch pgType {
	case 16:
		return MYSQL_TYPE_TINY
	case 20:
		return MYSQL_TYPE_LONGLONG
	case 21:
		return MYSQL_TYPE_SHORT
	case 23:
		return MYSQL_TYPE_LONG
	case 700:
		return MYSQL_TYPE_FLOAT
	case 701:
		return MYSQL_TYPE_DOUBLE
	case 1700:
		return MYSQL_TYPE_NEWDECIMAL
	case 1043, 25:
		return MYSQL_TYPE_VAR_STRING
	case 1042:
		return MYSQL_TYPE_STRING
	case 1082:
		return MYSQL_TYPE_DATE
	case 1083:
		return MYSQL_TYPE_TIME
	case 1114, 1184:
		return MYSQL_TYPE_DATETIME
	case 17:
		return MYSQL_TYPE_BLOB
	case 114, 3802:
		return MYSQL_TYPE_JSON
	default:
		return MYSQL_TYPE_VAR_STRING
	}
}

func (tm *TypeMapper) MySQLTypeToString(mysqlType byte) string {
	switch mysqlType {
	case MYSQL_TYPE_DECIMAL, MYSQL_TYPE_NEWDECIMAL:
		return "DECIMAL"
	case MYSQL_TYPE_TINY:
		return "TINYINT"
	case MYSQL_TYPE_SHORT:
		return "SMALLINT"
	case MYSQL_TYPE_LONG:
		return "INT"
	case MYSQL_TYPE_FLOAT:
		return "FLOAT"
	case MYSQL_TYPE_DOUBLE:
		return "DOUBLE"
	case MYSQL_TYPE_NULL:
		return "NULL"
	case MYSQL_TYPE_TIMESTAMP:
		return "TIMESTAMP"
	case MYSQL_TYPE_LONGLONG:
		return "BIGINT"
	case MYSQL_TYPE_INT24:
		return "MEDIUMINT"
	case MYSQL_TYPE_DATE:
		return "DATE"
	case MYSQL_TYPE_TIME:
		return "TIME"
	case MYSQL_TYPE_DATETIME:
		return "DATETIME"
	case MYSQL_TYPE_YEAR:
		return "YEAR"
	case MYSQL_TYPE_VARCHAR, MYSQL_TYPE_VAR_STRING:
		return "VARCHAR"
	case MYSQL_TYPE_BIT:
		return "BIT"
	case MYSQL_TYPE_JSON:
		return "JSON"
	case MYSQL_TYPE_ENUM:
		return "ENUM"
	case MYSQL_TYPE_SET:
		return "SET"
	case MYSQL_TYPE_TINY_BLOB:
		return "TINYBLOB"
	case MYSQL_TYPE_MEDIUM_BLOB:
		return "MEDIUMBLOB"
	case MYSQL_TYPE_LONG_BLOB:
		return "LONGBLOB"
	case MYSQL_TYPE_BLOB:
		return "BLOB"
	case MYSQL_TYPE_STRING:
		return "CHAR"
	case MYSQL_TYPE_GEOMETRY:
		return "GEOMETRY"
	default:
		return "UNKNOWN"
	}
}

func (tm *TypeMapper) ConvertValue(value interface{}, targetType byte) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	// Handle pgtype.Numeric specially
	if numeric, ok := value.(pgtype.Numeric); ok {
		if !numeric.Valid {
			return nil, nil
		}
		// Convert Numeric to float64
		float64Val, err := numeric.Float64Value()
		if err == nil && float64Val.Valid {
			value = float64Val.Float64
		} else {
			// Fallback to string representation
			value = numeric.Int.String()
		}
	}

	// Handle pgtype.Timestamp
	if ts, ok := value.(pgtype.Timestamp); ok {
		if !ts.Valid {
			return nil, nil
		}
		value = ts.Time
	}

	switch targetType {
	case MYSQL_TYPE_TINY, MYSQL_TYPE_SHORT, MYSQL_TYPE_LONG, MYSQL_TYPE_LONGLONG, MYSQL_TYPE_INT24:
		return tm.convertToInt(value)
	case MYSQL_TYPE_FLOAT, MYSQL_TYPE_DOUBLE:
		return tm.convertToFloat(value)
	case MYSQL_TYPE_DECIMAL, MYSQL_TYPE_NEWDECIMAL:
		return tm.convertToDecimal(value)
	case MYSQL_TYPE_VARCHAR, MYSQL_TYPE_VAR_STRING, MYSQL_TYPE_STRING:
		return tm.convertToString(value)
	case MYSQL_TYPE_DATE, MYSQL_TYPE_DATETIME, MYSQL_TYPE_TIMESTAMP:
		return tm.convertToTime(value)
	case MYSQL_TYPE_TIME:
		return tm.convertToTime(value)
	case MYSQL_TYPE_BLOB, MYSQL_TYPE_TINY_BLOB, MYSQL_TYPE_MEDIUM_BLOB, MYSQL_TYPE_LONG_BLOB:
		return tm.convertToBytes(value)
	case MYSQL_TYPE_JSON:
		return tm.convertToString(value)
	default:
		return value, nil
	}
}

func (tm *TypeMapper) convertToInt(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		var i int64
		_, err := fmt.Sscanf(v, "%d", &i)
		return i, err
	default:
		return value, nil
	}
}

func (tm *TypeMapper) convertToFloat(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		var f float64
		_, err := fmt.Sscanf(v, "%f", &f)
		return f, err
	default:
		return value, nil
	}
}

func (tm *TypeMapper) convertToDecimal(value interface{}) (interface{}, error) {
	return tm.convertToFloat(value)
}

func (tm *TypeMapper) convertToString(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return fmt.Sprintf("%v", value), nil
	}
}

func (tm *TypeMapper) convertToTime(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case []byte:
		// PostgreSQL sometimes returns timestamp as []byte
		str := string(v)
		t, err := time.Parse("2006-01-02 15:04:05", str)
		if err != nil {
			// Try parsing with timezone
			t, err = time.Parse("2006-01-02 15:04:05.999999", str)
			if err != nil {
				// Try date only
				t, err = time.Parse("2006-01-02", str)
				if err != nil {
					// Try with timezone offset
					t, err = time.Parse(time.RFC3339, str)
				}
			}
		}
		return t, err
	case string:
		t, err := time.Parse("2006-01-02 15:04:05", v)
		if err != nil {
			// Try parsing with microseconds
			t, err = time.Parse("2006-01-02 15:04:05.999999", v)
			if err != nil {
				// Try date only
				t, err = time.Parse("2006-01-02", v)
				if err != nil {
					// Try with timezone offset
					t, err = time.Parse(time.RFC3339, v)
				}
			}
		}
		return t, err
	default:
		return value, nil
	}
}

func (tm *TypeMapper) convertToBytes(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return []byte(fmt.Sprintf("%v", value)), nil
	}
}

func (tm *TypeMapper) FormatValueForMySQL(value interface{}, fieldType byte) (interface{}, error) {
	if value == nil {
		return nil, nil
	}

	switch fieldType {
	case MYSQL_TYPE_DATE:
		if t, ok := value.(time.Time); ok {
			return t.Format("2006-01-02"), nil
		}
		// Handle []byte from PostgreSQL
		if b, ok := value.([]byte); ok {
			str := string(b)
			// If it's already in the right format, return as string
			if len(str) == 10 && str[4] == '-' && str[7] == '-' {
				return str, nil
			}
			// Otherwise try to parse and reformat
			if t, err := time.Parse("2006-01-02", str); err == nil {
				return t.Format("2006-01-02"), nil
			}
		}
	case MYSQL_TYPE_DATETIME, MYSQL_TYPE_TIMESTAMP:
		if t, ok := value.(time.Time); ok {
			return t.Format("2006-01-02 15:04:05"), nil
		}
		// Handle []byte from PostgreSQL
		if b, ok := value.([]byte); ok {
			str := string(b)
			// Try to parse and reformat to MySQL format
			t, err := time.Parse("2006-01-02 15:04:05", str)
			if err != nil {
				// Try with microseconds
				t, err = time.Parse("2006-01-02 15:04:05.999999", str)
				if err != nil {
					// Try RFC3339
					t, err = time.Parse(time.RFC3339, str)
				}
			}
			if err == nil {
				return t.Format("2006-01-02 15:04:05"), nil
			}
			// If all parsing fails, return as-is
			return str, nil
		}
	case MYSQL_TYPE_TIME:
		if t, ok := value.(time.Time); ok {
			return t.Format("15:04:05"), nil
		}
		// Handle []byte from PostgreSQL
		if b, ok := value.([]byte); ok {
			str := string(b)
			if len(str) == 8 && str[2] == ':' && str[5] == ':' {
				return str, nil
			}
			if t, err := time.Parse("15:04:05", str); err == nil {
				return t.Format("15:04:05"), nil
			}
		}
	case MYSQL_TYPE_YEAR:
		if t, ok := value.(time.Time); ok {
			return t.Year(), nil
		}
	}

	return value, nil
}

func (tm *TypeMapper) BuildMySQLFieldPacket(name string, pgType uint32, length uint32) *mysql.Field {
	mysqlType := tm.PostgreSQLToMySQL(pgType)

	field := &mysql.Field{
		Name:    []byte(name),
		Type:    mysqlType,
		Charset: 33, // utf8_general_ci
	}

	if length > 0 {
		field.ColumnLength = length
	} else {
		field.ColumnLength = tm.GetDefaultLength(mysqlType)
	}

	return field
}

func (tm *TypeMapper) GetDefaultLength(mysqlType byte) uint32 {
	switch mysqlType {
	case MYSQL_TYPE_TINY:
		return 4
	case MYSQL_TYPE_SHORT:
		return 6
	case MYSQL_TYPE_LONG, MYSQL_TYPE_INT24:
		return 11
	case MYSQL_TYPE_LONGLONG:
		return 20
	case MYSQL_TYPE_FLOAT:
		return 12
	case MYSQL_TYPE_DOUBLE:
		return 22
	case MYSQL_TYPE_DECIMAL, MYSQL_TYPE_NEWDECIMAL:
		return 10
	case MYSQL_TYPE_DATE:
		return 10
	case MYSQL_TYPE_TIME:
		return 8
	case MYSQL_TYPE_DATETIME, MYSQL_TYPE_TIMESTAMP:
		return 19
	case MYSQL_TYPE_YEAR:
		return 4
	case MYSQL_TYPE_VARCHAR, MYSQL_TYPE_VAR_STRING:
		return 255
	case MYSQL_TYPE_STRING:
		return 255
	case MYSQL_TYPE_TINY_BLOB:
		return 255
	case MYSQL_TYPE_BLOB:
		return 65535
	case MYSQL_TYPE_MEDIUM_BLOB:
		return 16777215
	case MYSQL_TYPE_LONG_BLOB:
		return 4294967295
	case MYSQL_TYPE_JSON:
		return 4294967295
	default:
		return 255
	}
}
