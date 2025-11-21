package integration

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover MySQL-specific functions that have different syntax or don't exist in PostgreSQL

// Note: TestMySQLSpecific_MATCH_AGAINST has been moved to test/integration/mysql_specific_test.go
// because MATCH...AGAINST conversion is now supported via AST-based rewriting

// TestMySQLSpecific_FOUND_ROWS tests FOUND_ROWS() function
// PG Alternative: Use COUNT(*) OVER() or separate query
func TestMySQLSpecific_FOUND_ROWS(t *testing.T) {
	t.Skip("FOUND_ROWS() function not available in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_found_rows")
	db.Exec("CREATE TABLE test_found_rows (id INT PRIMARY KEY, val INT)")
	db.Exec("INSERT INTO test_found_rows VALUES (1,10), (2,20), (3,30), (4,40), (5,50)")
	defer db.Exec("DROP TABLE IF EXISTS test_found_rows")

	// SELECT with LIMIT
	db.Query("SELECT SQL_CALC_FOUND_ROWS * FROM test_found_rows LIMIT 2")

	// Get total rows without LIMIT
	var totalRows int
	err = db.QueryRow("SELECT FOUND_ROWS()").Scan(&totalRows)
	assert.NoError(t, err)
	assert.Equal(t, 5, totalRows)
}

// TestMySQLSpecific_GET_LOCK tests named locks
// PG Alternative: pg_advisory_lock()
func TestMySQLSpecific_GET_LOCK(t *testing.T) {
	t.Skip("GET_LOCK() function not available - use pg_advisory_lock() in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var lockResult int
	err = db.QueryRow("SELECT GET_LOCK('mylock', 10)").Scan(&lockResult)
	assert.NoError(t, err)
	assert.Equal(t, 1, lockResult) // 1 = success

	// Release lock
	var releaseResult int
	db.QueryRow("SELECT RELEASE_LOCK('mylock')").Scan(&releaseResult)
	assert.Equal(t, 1, releaseResult)
}

// TestMySQLSpecific_IS_FREE_LOCK tests lock status check
// PG Alternative: pg_advisory_unlock()
func TestMySQLSpecific_IS_FREE_LOCK(t *testing.T) {
	t.Skip("IS_FREE_LOCK() function not available in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var isFree int
	err = db.QueryRow("SELECT IS_FREE_LOCK('testlock')").Scan(&isFree)
	assert.NoError(t, err)
}

// TestMySQLSpecific_DATE_FORMAT tests DATE_FORMAT() function
// PG Alternative: TO_CHAR() with different format string syntax
func TestMySQLSpecific_DATE_FORMAT(t *testing.T) {
	t.Skip("DATE_FORMAT() syntax different - use TO_CHAR() in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var formatted string
	err = db.QueryRow("SELECT DATE_FORMAT(NOW(), '%Y-%m-%d %H:%i:%s')").Scan(&formatted)
	assert.NoError(t, err)
	assert.NotEmpty(t, formatted)
}

// TestMySQLSpecific_STR_TO_DATE tests STR_TO_DATE() function
// PG Alternative: TO_DATE() or TO_TIMESTAMP()
func TestMySQLSpecific_STR_TO_DATE(t *testing.T) {
	t.Skip("STR_TO_DATE() syntax different - use TO_DATE() in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var result string
	err = db.QueryRow("SELECT STR_TO_DATE('2024-01-15', '%Y-%m-%d')").Scan(&result)
	assert.NoError(t, err)
}

// TestMySQLSpecific_TIMESTAMPDIFF tests TIMESTAMPDIFF() function
// PG Alternative: EXTRACT(EPOCH FROM ...) or AGE()
func TestMySQLSpecific_TIMESTAMPDIFF(t *testing.T) {
	t.Skip("TIMESTAMPDIFF() not available - use EXTRACT(EPOCH FROM ...) in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var days int
	err = db.QueryRow("SELECT TIMESTAMPDIFF(DAY, '2024-01-01', '2024-01-15')").Scan(&days)
	assert.NoError(t, err)
	assert.Equal(t, 14, days)
}

// TestMySQLSpecific_GROUP_CONCAT_SEPARATOR tests GROUP_CONCAT with custom separator
// PG Alternative: string_agg(col, separator)
func TestMySQLSpecific_GROUP_CONCAT_SEPARATOR(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_group_concat")
	db.Exec("CREATE TABLE test_group_concat (id INT, name VARCHAR(50))")
	db.Exec("INSERT INTO test_group_concat VALUES (1, 'Alice'), (1, 'Bob'), (2, 'Charlie')")
	defer db.Exec("DROP TABLE IF EXISTS test_group_concat")

	// MySQL: GROUP_CONCAT with SEPARATOR
	// PG: string_agg(name, '|')
	var result string
	err = db.QueryRow("SELECT GROUP_CONCAT(name SEPARATOR '|') FROM test_group_concat WHERE id = 1").Scan(&result)
	assert.NoError(t, err)
	// Result should be "Alice|Bob" or "Bob|Alice" depending on order
	assert.Contains(t, result, "Alice")
	assert.Contains(t, result, "Bob")
}

// TestMySQLSpecific_ENCRYPT tests ENCRYPT() function
// PG Alternative: pgcrypto extension
func TestMySQLSpecific_ENCRYPT(t *testing.T) {
	t.Skip("ENCRYPT() function not available - use pgcrypto extension in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var encrypted string
	err = db.QueryRow("SELECT ENCRYPT('password', 'salt')").Scan(&encrypted)
	// Note: ENCRYPT() was deprecated and removed in MySQL 8.0.3
	t.Log("ENCRYPT result:", encrypted, err)
}

// TestMySQLSpecific_PASSWORD tests PASSWORD() function
// Deprecated and removed in MySQL 8.0
func TestMySQLSpecific_PASSWORD(t *testing.T) {
	t.Skip("PASSWORD() function deprecated in MySQL and not in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var hashed string
	err = db.QueryRow("SELECT PASSWORD('secret')").Scan(&hashed)
	// This will likely fail in MySQL 8.0+
	t.Log("PASSWORD result:", hashed, err)
}

// TestMySQLSpecific_LAST_INSERT_ID tests LAST_INSERT_ID() function
// PG Alternative: RETURNING clause or currval()
// NOTE: This is now supported via session state tracking
func TestMySQLSpecific_LAST_INSERT_ID(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_last_id")
	_, err = db.Exec("CREATE TABLE test_last_id (id INT AUTO_INCREMENT PRIMARY KEY, val INT)")
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_last_id")

	// Test 1: Insert single row and verify LastInsertId() from result
	result, err := db.Exec("INSERT INTO test_last_id (val) VALUES (100)")
	require.NoError(t, err)

	lastIDFromResult, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Greater(t, lastIDFromResult, int64(0))

	// Test 2: Verify LAST_INSERT_ID() function matches result.LastInsertId()
	var lastIDFromFunc int64
	err = db.QueryRow("SELECT LAST_INSERT_ID()").Scan(&lastIDFromFunc)
	assert.NoError(t, err)
	assert.Equal(t, lastIDFromResult, lastIDFromFunc, "LAST_INSERT_ID() should match result.LastInsertId()")

	// Test 3: Insert 1000 rows and verify last insert ID after batch
	for i := 0; i < 1000; i++ {
		result, err = db.Exec("INSERT INTO test_last_id (val) VALUES (?)", 200+i)
		require.NoError(t, err)
	}

	// Get the last insert ID from the result of the 1000th insert
	lastIDAfter1000, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Equal(t, lastIDFromResult+1000, lastIDAfter1000, "After 1000 inserts, last ID should be first ID + 1000")

	// Test 4: Verify LAST_INSERT_ID() function returns the last inserted ID
	var lastIDFromFunc2 int64
	err = db.QueryRow("SELECT LAST_INSERT_ID()").Scan(&lastIDFromFunc2)
	assert.NoError(t, err)
	assert.Equal(t, lastIDAfter1000, lastIDFromFunc2, "LAST_INSERT_ID() should return the last inserted ID after 1000 inserts")

	// Test 5: Insert a batch of 500 more rows
	for i := 0; i < 500; i++ {
		result, err = db.Exec("INSERT INTO test_last_id (val) VALUES (?)", 1200+i)
		require.NoError(t, err)
	}

	lastIDAfter1500, err := result.LastInsertId()
	require.NoError(t, err)
	assert.Equal(t, lastIDFromResult+1500, lastIDAfter1500, "After 1500 total inserts, last ID should be first ID + 1500")

	// Test 6: Verify LAST_INSERT_ID() still works after more inserts
	var lastIDFromFunc3 int64
	err = db.QueryRow("SELECT LAST_INSERT_ID()").Scan(&lastIDFromFunc3)
	assert.NoError(t, err)
	assert.Equal(t, lastIDAfter1500, lastIDFromFunc3, "LAST_INSERT_ID() should return the latest inserted ID")

	// Test 7: Verify the total count is 1501 (1 initial + 1500 batch)
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_last_id").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1501, count, "Should have 1501 total rows")
}

// TestMySQLSpecific_FORMAT tests FORMAT() function for number formatting
// PG Alternative: TO_CHAR() with format pattern
func TestMySQLSpecific_FORMAT(t *testing.T) {
	t.Skip("FORMAT() for numbers has different syntax - use TO_CHAR() in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var formatted string
	err = db.QueryRow("SELECT FORMAT(1234567.89, 2)").Scan(&formatted)
	assert.NoError(t, err)
	assert.Equal(t, "1,234,567.89", formatted)
}

// TestMySQLSpecific_INET_ATON tests IP address conversion
// PG Alternative: inet datatype with built-in operators
func TestMySQLSpecific_INET_ATON(t *testing.T) {
	t.Skip("INET_ATON() not available - use inet type in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var ipNum int64
	err = db.QueryRow("SELECT INET_ATON('192.168.1.1')").Scan(&ipNum)
	assert.NoError(t, err)
}

// TestMySQLSpecific_INET_NTOA tests IP address conversion (reverse)
func TestMySQLSpecific_INET_NTOA(t *testing.T) {
	t.Skip("INET_NTOA() not available - use inet type in PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var ip string
	err = db.QueryRow("SELECT INET_NTOA(3232235777)").Scan(&ip) // 192.168.1.1
	assert.NoError(t, err)
}

// TestMySQLSpecific_LOAD_FILE tests LOAD_FILE() function
// Security risk, often disabled
func TestMySQLSpecific_LOAD_FILE(t *testing.T) {
	t.Skip("LOAD_FILE() function not available in PostgreSQL and is a security risk")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	var content sql.NullString
	err = db.QueryRow("SELECT LOAD_FILE('/etc/passwd')").Scan(&content)
	// Will likely fail due to permissions
	t.Log("LOAD_FILE result:", content.Valid, err)
}
