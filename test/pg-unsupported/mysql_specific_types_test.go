package integration

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover MySQL-specific data types that PostgreSQL does NOT support
// These features require application-level changes to migrate

const proxyDSN = "root@tcp(127.0.0.1:3306)/test"

// TestMySQLSpecific_ENUM tests ENUM type which PG doesn't support natively
// PG Alternative: CREATE TYPE custom_enum AS ENUM (...)
func TestMySQLSpecific_ENUM(t *testing.T) {
	t.Skip("ENUM type not supported by PostgreSQL - requires custom TYPE definition")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_enum")
	_, err = db.Exec(`CREATE TABLE test_enum (
		id INT AUTO_INCREMENT PRIMARY KEY,
		status ENUM('active', 'inactive', 'pending')
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_enum")

	// Test INSERT
	_, err = db.Exec("INSERT INTO test_enum (status) VALUES ('active'), ('inactive')")
	assert.NoError(t, err)

	// Test SELECT
	var status string
	err = db.QueryRow("SELECT status FROM test_enum WHERE id = 1").Scan(&status)
	assert.NoError(t, err)
	assert.Equal(t, "active", status)
}

// TestMySQLSpecific_SET tests SET type which PG doesn't support
// PG Alternative: TEXT[] array or bit string
func TestMySQLSpecific_SET(t *testing.T) {
	t.Skip("SET type not supported by PostgreSQL - use TEXT[] array instead")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_set")
	_, err = db.Exec(`CREATE TABLE test_set (
		id INT AUTO_INCREMENT PRIMARY KEY,
		permissions SET('read', 'write', 'execute')
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_set")

	_, err = db.Exec("INSERT INTO test_set (permissions) VALUES ('read,write')")
	assert.NoError(t, err)
}

// TestMySQLSpecific_TINYINT1_AsBoolean tests TINYINT(1) as boolean
// PG has proper BOOLEAN type
func TestMySQLSpecific_TINYINT1_AsBoolean(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_bool")
	_, err = db.Exec(`CREATE TABLE test_bool (
		id INT AUTO_INCREMENT PRIMARY KEY,
		is_active TINYINT(1)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_bool")

	// Note: In PG this should be BOOLEAN type
	_, err = db.Exec("INSERT INTO test_bool (is_active) VALUES (1), (0)")
	assert.NoError(t, err)

	var isActive int
	err = db.QueryRow("SELECT is_active FROM test_bool WHERE id = 1").Scan(&isActive)
	assert.NoError(t, err)
	assert.Equal(t, 1, isActive)
}

// TestMySQLSpecific_MEDIUMINT tests MEDIUMINT which doesn't exist in PG
// PG Alternative: INT
func TestMySQLSpecific_MEDIUMINT(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_mediumint")
	_, err = db.Exec(`CREATE TABLE test_mediumint (
		id INT AUTO_INCREMENT PRIMARY KEY,
		medium_val MEDIUMINT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_mediumint")

	// MEDIUMINT range: -8388608 to 8388607
	_, err = db.Exec("INSERT INTO test_mediumint (medium_val) VALUES (8388607)")
	assert.NoError(t, err)

	var val int
	err = db.QueryRow("SELECT medium_val FROM test_mediumint WHERE id = 1").Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, 8388607, val)
}

// TestMySQLSpecific_DisplayWidth tests integer display width like INT(11)
// PG doesn't support display width - it's purely cosmetic in MySQL
func TestMySQLSpecific_DisplayWidth(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_display_width")
	_, err = db.Exec(`CREATE TABLE test_display_width (
		id INT AUTO_INCREMENT PRIMARY KEY,
		val INT(11),
		zeropad INT(5) ZEROFILL
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_display_width")

	// Display width is cosmetic and should be ignored in PG
	_, err = db.Exec("INSERT INTO test_display_width (val, zeropad) VALUES (123, 456)")
	assert.NoError(t, err)
}

// TestMySQLSpecific_SpatialTypes tests GEOMETRY, POINT, etc.
// PG Alternative: PostGIS extension
func TestMySQLSpecific_SpatialTypes(t *testing.T) {
	t.Skip("MySQL SPATIAL types not supported - use PostGIS extension in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_spatial")
	_, err = db.Exec(`CREATE TABLE test_spatial (
		id INT AUTO_INCREMENT PRIMARY KEY,
		location POINT,
		area POLYGON
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_spatial")
}

// TestMySQLSpecific_DataTypes_Combined tests multiple unsupported types together
func TestMySQLSpecific_DataTypes_Combined(t *testing.T) {
	t.Skip("This test contains multiple PostgreSQL-unsupported types")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_combined")
	_, err = db.Exec(`CREATE TABLE test_combined (
		id INT AUTO_INCREMENT PRIMARY KEY,
		tiny_col TINYINT,
		small_col SMALLINT,
		medium_col MEDIUMINT,
		int_col INT,
		big_col BIGINT,
		float_col FLOAT,
		double_col DOUBLE,
		decimal_col DECIMAL(10,2),
		char_col CHAR(10),
		varchar_col VARCHAR(100),
		text_col TEXT,
		date_col DATE,
		datetime_col DATETIME,
		timestamp_col TIMESTAMP,
		year_col YEAR,
		enum_col ENUM('a', 'b', 'c'),
		set_col SET('x', 'y', 'z'),
		json_col JSON
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_combined")

	now := time.Now()
	_, err = db.Exec(`INSERT INTO test_combined (
		tiny_col, small_col, medium_col, int_col, big_col,
		float_col, double_col, decimal_col,
		char_col, varchar_col, text_col,
		date_col, datetime_col, timestamp_col, year_col,
		enum_col, set_col, json_col
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		127, 32767, 8388607, 2147483647, 9223372036854775807,
		3.14, 2.718281828, 12345.67,
		"char", "varchar", "text content",
		now.Format("2006-01-02"), now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"), 2024,
		"a", "x,y", `{"key":"value"}`,
	)
	assert.NoError(t, err)
}
