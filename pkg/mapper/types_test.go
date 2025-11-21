package mapper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTypeMapper_PostgreSQLToMySQL(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
	}{
		{"boolean", 16, MYSQL_TYPE_TINY},
		{"bigint", 20, MYSQL_TYPE_LONGLONG},
		{"smallint", 21, MYSQL_TYPE_SHORT},
		{"integer", 23, MYSQL_TYPE_LONG},
		{"real", 700, MYSQL_TYPE_FLOAT},
		{"double precision", 701, MYSQL_TYPE_DOUBLE},
		{"numeric", 1700, MYSQL_TYPE_NEWDECIMAL},
		{"varchar", 1043, MYSQL_TYPE_VAR_STRING},
		{"text", 25, MYSQL_TYPE_VAR_STRING},
		{"char", 1042, MYSQL_TYPE_STRING},
		{"date", 1082, MYSQL_TYPE_DATE},
		{"time", 1083, MYSQL_TYPE_TIME},
		{"timestamp", 1114, MYSQL_TYPE_DATETIME},
		{"bytea", 17, MYSQL_TYPE_BLOB},
		{"jsonb", 3802, MYSQL_TYPE_JSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, result)
		})
	}
}

func TestTypeMapper_MySQLTypeToString(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		mysqlType byte
		expected  string
	}{
		{MYSQL_TYPE_DECIMAL, "DECIMAL"},
		{MYSQL_TYPE_TINY, "TINYINT"},
		{MYSQL_TYPE_SHORT, "SMALLINT"},
		{MYSQL_TYPE_LONG, "INT"},
		{MYSQL_TYPE_FLOAT, "FLOAT"},
		{MYSQL_TYPE_DOUBLE, "DOUBLE"},
		{MYSQL_TYPE_LONGLONG, "BIGINT"},
		{MYSQL_TYPE_DATE, "DATE"},
		{MYSQL_TYPE_TIME, "TIME"},
		{MYSQL_TYPE_DATETIME, "DATETIME"},
		{MYSQL_TYPE_VARCHAR, "VARCHAR"},
		{MYSQL_TYPE_JSON, "JSON"},
		{MYSQL_TYPE_BLOB, "BLOB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tm.MySQLTypeToString(tt.mysqlType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_ConvertValue(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
		expected   interface{}
	}{
		{"int to int", 42, MYSQL_TYPE_LONG, int64(42)},
		{"string to int", "42", MYSQL_TYPE_LONG, int64(42)},
		{"float to int", 42.7, MYSQL_TYPE_LONG, int64(42)},
		{"int to float", 42, MYSQL_TYPE_FLOAT, float64(42)},
		{"string to float", "42.5", MYSQL_TYPE_DOUBLE, float64(42.5)},
		{"bytes to string", []byte("hello"), MYSQL_TYPE_VAR_STRING, "hello"},
		{"string to bytes", "hello", MYSQL_TYPE_BLOB, []byte("hello")},
		{"nil value", nil, MYSQL_TYPE_LONG, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.ConvertValue(tt.value, tt.targetType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_FormatValueForMySQL(t *testing.T) {
	tm := NewTypeMapper()

	now := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name      string
		value     interface{}
		fieldType byte
		expected  interface{}
	}{
		{"date formatting", now, MYSQL_TYPE_DATE, "2024-01-15"},
		{"datetime formatting", now, MYSQL_TYPE_DATETIME, "2024-01-15 14:30:45"},
		{"time formatting", now, MYSQL_TYPE_TIME, "14:30:45"},
		{"year formatting", now, MYSQL_TYPE_YEAR, 2024},
		{"string passthrough", "test", MYSQL_TYPE_VAR_STRING, "test"},
		{"nil value", nil, MYSQL_TYPE_DATE, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.FormatValueForMySQL(tt.value, tt.fieldType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_GetDefaultLength(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		mysqlType byte
		expected  uint32
	}{
		{MYSQL_TYPE_TINY, 4},
		{MYSQL_TYPE_SHORT, 6},
		{MYSQL_TYPE_LONG, 11},
		{MYSQL_TYPE_LONGLONG, 20},
		{MYSQL_TYPE_FLOAT, 12},
		{MYSQL_TYPE_DOUBLE, 22},
		{MYSQL_TYPE_DATE, 10},
		{MYSQL_TYPE_TIME, 8},
		{MYSQL_TYPE_DATETIME, 19},
		{MYSQL_TYPE_VARCHAR, 255},
		{MYSQL_TYPE_BLOB, 65535},
	}

	for _, tt := range tests {
		t.Run(tm.MySQLTypeToString(tt.mysqlType), func(t *testing.T) {
			result := tm.GetDefaultLength(tt.mysqlType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ============================================================================
// Comprehensive Type Mapping Test Cases
// Based on docs/MYSQL_TO_PG_CASES.md
// ============================================================================

func TestTypeMapper_IntegerTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name        string
		pgType      uint32
		pgTypeName  string
		mysqlType   byte
		mysqlName   string
	}{
		{"boolean", 16, "bool", MYSQL_TYPE_TINY, "TINYINT"},
		{"smallint", 21, "int2", MYSQL_TYPE_SHORT, "SMALLINT"},
		{"integer", 23, "int4", MYSQL_TYPE_LONG, "INT"},
		{"bigint", 20, "int8", MYSQL_TYPE_LONGLONG, "BIGINT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test PG to MySQL type conversion
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType, "PG type %d should map to MySQL type %d", tt.pgType, tt.mysqlType)

			// Test MySQL type to string
			mysqlName := tm.MySQLTypeToString(tt.mysqlType)
			assert.Equal(t, tt.mysqlName, mysqlName, "MySQL type %d should be named %s", tt.mysqlType, tt.mysqlName)
		})
	}
}

func TestTypeMapper_FloatingPointTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
		mysqlName  string
	}{
		{"real/float4", 700, MYSQL_TYPE_FLOAT, "FLOAT"},
		{"double precision/float8", 701, MYSQL_TYPE_DOUBLE, "DOUBLE"},
		{"numeric/decimal", 1700, MYSQL_TYPE_NEWDECIMAL, "DECIMAL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType)

			mysqlName := tm.MySQLTypeToString(tt.mysqlType)
			assert.Contains(t, mysqlName, tt.mysqlName)
		})
	}
}

func TestTypeMapper_StringTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
	}{
		{"varchar", 1043, MYSQL_TYPE_VAR_STRING},
		{"char/bpchar", 1042, MYSQL_TYPE_STRING},
		{"text", 25, MYSQL_TYPE_VAR_STRING},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType)
		})
	}
}

func TestTypeMapper_DateTimeTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
		mysqlName  string
	}{
		{"date", 1082, MYSQL_TYPE_DATE, "DATE"},
		{"time", 1083, MYSQL_TYPE_TIME, "TIME"},
		{"timestamp", 1114, MYSQL_TYPE_DATETIME, "DATETIME"},
		{"timestamptz", 1184, MYSQL_TYPE_DATETIME, "DATETIME"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType)

			mysqlName := tm.MySQLTypeToString(tt.mysqlType)
			assert.Equal(t, tt.mysqlName, mysqlName)
		})
	}
}

func TestTypeMapper_BinaryTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
	}{
		{"bytea", 17, MYSQL_TYPE_BLOB},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType)
		})
	}
}

func TestTypeMapper_JSONTypes_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		pgType     uint32
		mysqlType  byte
	}{
		{"json", 114, MYSQL_TYPE_JSON},
		{"jsonb", 3802, MYSQL_TYPE_JSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mysqlType := tm.PostgreSQLToMySQL(tt.pgType)
			assert.Equal(t, tt.mysqlType, mysqlType)
		})
	}
}

func TestTypeMapper_ConvertValue_Integers(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
		expected   interface{}
		wantErr    bool
	}{
		// TINYINT conversions
		{"int to tinyint", 42, MYSQL_TYPE_TINY, int64(42), false},
		{"string to tinyint", "42", MYSQL_TYPE_TINY, int64(42), false},
		{"negative to tinyint", -10, MYSQL_TYPE_TINY, int64(-10), false},

		// SMALLINT conversions
		{"int to smallint", 1000, MYSQL_TYPE_SHORT, int64(1000), false},
		{"string to smallint", "1000", MYSQL_TYPE_SHORT, int64(1000), false},

		// INT conversions
		{"int to int", 100000, MYSQL_TYPE_LONG, int64(100000), false},
		{"string to int", "100000", MYSQL_TYPE_LONG, int64(100000), false},
		{"float to int", 42.9, MYSQL_TYPE_LONG, int64(42), false},

		// BIGINT conversions
		{"int to bigint", 9223372036854775807, MYSQL_TYPE_LONGLONG, int64(9223372036854775807), false},
		{"string to bigint", "9223372036854775807", MYSQL_TYPE_LONGLONG, int64(9223372036854775807), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.ConvertValue(tt.value, tt.targetType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestTypeMapper_ConvertValue_FloatingPoint(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
		expected   interface{}
	}{
		// FLOAT conversions
		{"int to float", 42, MYSQL_TYPE_FLOAT, float64(42)},
		{"string to float", "42.5", MYSQL_TYPE_FLOAT, float64(42.5)},
		{"float to float", 3.14159, MYSQL_TYPE_FLOAT, float64(3.14159)},

		// DOUBLE conversions
		{"int to double", 42, MYSQL_TYPE_DOUBLE, float64(42)},
		{"string to double", "3.14159265359", MYSQL_TYPE_DOUBLE, float64(3.14159265359)},
		{"float to double", 3.14, MYSQL_TYPE_DOUBLE, float64(3.14)},

		// DECIMAL conversions
		{"int to decimal", 42, MYSQL_TYPE_NEWDECIMAL, float64(42)},
		{"string to decimal", "123.45", MYSQL_TYPE_NEWDECIMAL, float64(123.45)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.ConvertValue(tt.value, tt.targetType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_ConvertValue_Strings(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
		expected   interface{}
	}{
		// VARCHAR conversions
		{"string to varchar", "hello", MYSQL_TYPE_VAR_STRING, "hello"},
		{"bytes to varchar", []byte("hello"), MYSQL_TYPE_VAR_STRING, "hello"},
		{"int to varchar", 42, MYSQL_TYPE_VAR_STRING, "42"},

		// CHAR conversions
		{"string to char", "test", MYSQL_TYPE_STRING, "test"},
		{"bytes to char", []byte("test"), MYSQL_TYPE_STRING, "test"},

		// TEXT conversions
		{"string to text", "long text content", MYSQL_TYPE_BLOB, []byte("long text content")},
		{"bytes to text", []byte("long text"), MYSQL_TYPE_BLOB, []byte("long text")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.ConvertValue(tt.value, tt.targetType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_ConvertValue_DateTime(t *testing.T) {
	tm := NewTypeMapper()

	testTime := time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC)

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
	}{
		{"time.Time to DATE", testTime, MYSQL_TYPE_DATE},
		{"time.Time to TIME", testTime, MYSQL_TYPE_TIME},
		{"time.Time to DATETIME", testTime, MYSQL_TYPE_DATETIME},
		{"time.Time to TIMESTAMP", testTime, MYSQL_TYPE_TIMESTAMP},
		{"time.Time to YEAR", testTime, MYSQL_TYPE_YEAR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tm.ConvertValue(tt.value, tt.targetType)
			assert.NoError(t, err)
		})
	}
}

func TestTypeMapper_ConvertValue_JSON(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		name       string
		value      interface{}
		targetType byte
		expected   interface{}
	}{
		{"JSON object", `{"key": "value"}`, MYSQL_TYPE_JSON, `{"key": "value"}`},
		{"JSON array", `[1, 2, 3]`, MYSQL_TYPE_JSON, `[1, 2, 3]`},
		{"bytes to JSON", []byte(`{"test": true}`), MYSQL_TYPE_JSON, `{"test": true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.ConvertValue(tt.value, tt.targetType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_ConvertValue_NullValues(t *testing.T) {
	tm := NewTypeMapper()

	types := []byte{
		MYSQL_TYPE_TINY,
		MYSQL_TYPE_SHORT,
		MYSQL_TYPE_LONG,
		MYSQL_TYPE_LONGLONG,
		MYSQL_TYPE_FLOAT,
		MYSQL_TYPE_DOUBLE,
		MYSQL_TYPE_NEWDECIMAL,
		MYSQL_TYPE_VAR_STRING,
		MYSQL_TYPE_STRING,
		MYSQL_TYPE_BLOB,
		MYSQL_TYPE_DATE,
		MYSQL_TYPE_TIME,
		MYSQL_TYPE_DATETIME,
		MYSQL_TYPE_JSON,
	}

	for _, mysqlType := range types {
		t.Run(tm.MySQLTypeToString(mysqlType), func(t *testing.T) {
			result, err := tm.ConvertValue(nil, mysqlType)
			assert.NoError(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestTypeMapper_FormatValueForMySQL_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	testTime := time.Date(2024, 12, 25, 15, 30, 45, 0, time.UTC)

	tests := []struct {
		name      string
		value     interface{}
		fieldType byte
		expected  interface{}
	}{
		// Date formatting
		{"date YYYY-MM-DD", testTime, MYSQL_TYPE_DATE, "2024-12-25"},
		{"datetime YYYY-MM-DD HH:MM:SS", testTime, MYSQL_TYPE_DATETIME, "2024-12-25 15:30:45"},
		{"time HH:MM:SS", testTime, MYSQL_TYPE_TIME, "15:30:45"},
		{"timestamp", testTime, MYSQL_TYPE_TIMESTAMP, "2024-12-25 15:30:45"},
		{"year", testTime, MYSQL_TYPE_YEAR, 2024},

		// String types
		{"varchar", "test string", MYSQL_TYPE_VAR_STRING, "test string"},
		{"char", "fixed", MYSQL_TYPE_STRING, "fixed"},

		// Numeric types
		{"tinyint", int64(42), MYSQL_TYPE_TINY, int64(42)},
		{"smallint", int64(1000), MYSQL_TYPE_SHORT, int64(1000)},
		{"int", int64(100000), MYSQL_TYPE_LONG, int64(100000)},
		{"bigint", int64(9223372036854775807), MYSQL_TYPE_LONGLONG, int64(9223372036854775807)},
		{"float", 3.14, MYSQL_TYPE_FLOAT, 3.14},
		{"double", 3.14159265359, MYSQL_TYPE_DOUBLE, 3.14159265359},

		// Binary types
		{"blob", []byte("binary data"), MYSQL_TYPE_BLOB, []byte("binary data")},

		// JSON types
		{"json", `{"key": "value"}`, MYSQL_TYPE_JSON, `{"key": "value"}`},

		// Null values
		{"null date", nil, MYSQL_TYPE_DATE, nil},
		{"null string", nil, MYSQL_TYPE_VAR_STRING, nil},
		{"null int", nil, MYSQL_TYPE_LONG, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tm.FormatValueForMySQL(tt.value, tt.fieldType)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_EdgeCases(t *testing.T) {
	tm := NewTypeMapper()

	t.Run("unknown PostgreSQL type", func(t *testing.T) {
		result := tm.PostgreSQLToMySQL(99999)
		assert.Equal(t, byte(MYSQL_TYPE_VAR_STRING), result, "Unknown types should default to VARCHAR")
	})

	t.Run("unknown MySQL type to string", func(t *testing.T) {
		result := tm.MySQLTypeToString(0x11) // Use an undefined type value
		assert.Equal(t, "UNKNOWN", result)
	})

	t.Run("invalid string to int conversion", func(t *testing.T) {
		_, err := tm.ConvertValue("not a number", MYSQL_TYPE_LONG)
		assert.Error(t, err)
	})

	t.Run("invalid string to float conversion", func(t *testing.T) {
		_, err := tm.ConvertValue("not a float", MYSQL_TYPE_FLOAT)
		assert.Error(t, err)
	})

	t.Run("empty string conversions", func(t *testing.T) {
		result, err := tm.ConvertValue("", MYSQL_TYPE_VAR_STRING)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("zero values", func(t *testing.T) {
		result, err := tm.ConvertValue(0, MYSQL_TYPE_LONG)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), result)
	})
}

func TestTypeMapper_GetDefaultLength_Comprehensive(t *testing.T) {
	tm := NewTypeMapper()

	tests := []struct {
		mysqlType byte
		expected  uint32
	}{
		// Integer types
		{MYSQL_TYPE_TINY, 4},
		{MYSQL_TYPE_SHORT, 6},
		{MYSQL_TYPE_LONG, 11},
		{MYSQL_TYPE_LONGLONG, 20},

		// Floating point types
		{MYSQL_TYPE_FLOAT, 12},
		{MYSQL_TYPE_DOUBLE, 22},
		{MYSQL_TYPE_NEWDECIMAL, 10},

		// Date/time types
		{MYSQL_TYPE_DATE, 10},
		{MYSQL_TYPE_TIME, 8},
		{MYSQL_TYPE_DATETIME, 19},
		{MYSQL_TYPE_TIMESTAMP, 19},
		{MYSQL_TYPE_YEAR, 4},

		// String types
		{MYSQL_TYPE_VAR_STRING, 255},
		{MYSQL_TYPE_VARCHAR, 255},
		{MYSQL_TYPE_STRING, 255},

		// Binary types
		{MYSQL_TYPE_BLOB, 65535},
		{MYSQL_TYPE_TINY_BLOB, 255},
		{MYSQL_TYPE_MEDIUM_BLOB, 16777215},
		{MYSQL_TYPE_LONG_BLOB, 4294967295},

		// Other types
		{MYSQL_TYPE_JSON, 4294967295},
	}

	for _, tt := range tests {
		t.Run(tm.MySQLTypeToString(tt.mysqlType), func(t *testing.T) {
			result := tm.GetDefaultLength(tt.mysqlType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTypeMapper_ConcurrentAccess(t *testing.T) {
	tm := NewTypeMapper()

	// Test concurrent access to ensure thread safety
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				tm.PostgreSQLToMySQL(23)
				tm.MySQLTypeToString(MYSQL_TYPE_LONG)
				tm.ConvertValue(42, MYSQL_TYPE_LONG)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkTypeMapper_PostgreSQLToMySQL(b *testing.B) {
	tm := NewTypeMapper()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.PostgreSQLToMySQL(23)
	}
}

func BenchmarkTypeMapper_ConvertValue(b *testing.B) {
	tm := NewTypeMapper()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.ConvertValue(42, MYSQL_TYPE_LONG)
	}
}

func BenchmarkTypeMapper_FormatValueForMySQL(b *testing.B) {
	tm := NewTypeMapper()
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.FormatValueForMySQL(now, MYSQL_TYPE_DATETIME)
	}
}
