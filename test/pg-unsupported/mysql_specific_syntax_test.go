package integration

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests cover MySQL-specific SQL syntax that PostgreSQL does NOT support

// TestMySQLSpecific_REPLACE_INTO tests REPLACE INTO statement
// PG Alternative: INSERT ... ON CONFLICT but semantics are different
// REPLACE deletes then inserts, while ON CONFLICT updates
func TestMySQLSpecific_REPLACE_INTO(t *testing.T) {
	t.Skip("REPLACE INTO not fully compatible with PostgreSQL - use INSERT ... ON CONFLICT")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_replace")
	_, err = db.Exec(`CREATE TABLE test_replace (
		id INT PRIMARY KEY,
		name VARCHAR(100),
		count INT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_replace")

	// Initial insert
	_, err = db.Exec("INSERT INTO test_replace VALUES (1, 'Alice', 10)")
	require.NoError(t, err)

	// REPLACE will delete and re-insert
	_, err = db.Exec("REPLACE INTO test_replace VALUES (1, 'Alice Updated', 20)")
	assert.NoError(t, err)

	var name string
	var count int
	err = db.QueryRow("SELECT name, count FROM test_replace WHERE id = 1").Scan(&name, &count)
	assert.NoError(t, err)
	assert.Equal(t, "Alice Updated", name)
	assert.Equal(t, 20, count)
}

// TestMySQLSpecific_INSERT_VALUES_Function tests VALUES() function in ON DUPLICATE KEY UPDATE
// PG Alternative: Use EXCLUDED table reference
func TestMySQLSpecific_INSERT_VALUES_Function(t *testing.T) {
	t.Skip("VALUES() function in ON DUPLICATE KEY UPDATE not supported - use EXCLUDED in PG")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_values_func")
	_, err = db.Exec(`CREATE TABLE test_values_func (
		id INT PRIMARY KEY,
		count INT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_values_func")

	// This syntax is MySQL-specific
	_, err = db.Exec(`
		INSERT INTO test_values_func (id, count) VALUES (1, 10)
		ON DUPLICATE KEY UPDATE count = count + VALUES(count)
	`)
	assert.NoError(t, err)

	// Run again - should increment
	_, err = db.Exec(`
		INSERT INTO test_values_func (id, count) VALUES (1, 5)
		ON DUPLICATE KEY UPDATE count = count + VALUES(count)
	`)
	assert.NoError(t, err)

	var count int
	err = db.QueryRow("SELECT count FROM test_values_func WHERE id = 1").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 15, count) // 10 + 5
}

// TestMySQLSpecific_UPDATE_LIMIT tests UPDATE with LIMIT
// PG doesn't support LIMIT in UPDATE statements
func TestMySQLSpecific_UPDATE_LIMIT(t *testing.T) {
	t.Skip("UPDATE ... LIMIT not supported by PostgreSQL - use subquery instead")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_update_limit")
	_, err = db.Exec(`CREATE TABLE test_update_limit (
		id INT AUTO_INCREMENT PRIMARY KEY,
		status VARCHAR(20)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_update_limit")

	// Insert test data
	db.Exec("INSERT INTO test_update_limit (status) VALUES ('pending'), ('pending'), ('pending')")

	// Update only first row
	result, err := db.Exec("UPDATE test_update_limit SET status = 'done' WHERE status = 'pending' LIMIT 1")
	assert.NoError(t, err)

	affected, _ := result.RowsAffected()
	assert.Equal(t, int64(1), affected)
}

// TestMySQLSpecific_DELETE_LIMIT tests DELETE with LIMIT
// PG doesn't support LIMIT in DELETE statements
func TestMySQLSpecific_DELETE_LIMIT(t *testing.T) {
	t.Skip("DELETE ... LIMIT not supported by PostgreSQL - use subquery with LIMIT")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_delete_limit")
	_, err = db.Exec(`CREATE TABLE test_delete_limit (
		id INT AUTO_INCREMENT PRIMARY KEY,
		value INT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_delete_limit")

	// Insert test data
	db.Exec("INSERT INTO test_delete_limit (value) VALUES (1), (2), (3), (4), (5)")

	// Delete only 2 rows
	result, err := db.Exec("DELETE FROM test_delete_limit WHERE value > 0 LIMIT 2")
	assert.NoError(t, err)

	affected, _ := result.RowsAffected()
	assert.Equal(t, int64(2), affected)

	// Should have 3 rows left
	var count int
	db.QueryRow("SELECT COUNT(*) FROM test_delete_limit").Scan(&count)
	assert.Equal(t, 3, count)
}

// TestMySQLSpecific_STRAIGHT_JOIN tests STRAIGHT_JOIN hint
// PG Alternative: Use explicit JOIN order or pg_hint_plan extension
func TestMySQLSpecific_STRAIGHT_JOIN(t *testing.T) {
	t.Skip("STRAIGHT_JOIN hint not supported by PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_table1, test_table2")
	db.Exec("CREATE TABLE test_table1 (id INT PRIMARY KEY, name VARCHAR(50))")
	db.Exec("CREATE TABLE test_table2 (id INT PRIMARY KEY, ref_id INT)")
	defer db.Exec("DROP TABLE IF EXISTS test_table1, test_table2")

	// STRAIGHT_JOIN forces join order
	rows, err := db.Query(`
		SELECT t1.name, t2.id
		FROM test_table1 t1
		STRAIGHT_JOIN test_table2 t2 ON t1.id = t2.ref_id
	`)
	assert.NoError(t, err)
	if rows != nil {
		rows.Close()
	}
}

// TestMySQLSpecific_FORCE_INDEX tests FORCE INDEX hint
// PG Alternative: Use pg_hint_plan extension or rewrite query
func TestMySQLSpecific_FORCE_INDEX(t *testing.T) {
	t.Skip("FORCE INDEX hint not supported by PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_force_index")
	_, err = db.Exec(`CREATE TABLE test_force_index (
		id INT PRIMARY KEY,
		name VARCHAR(50),
		age INT,
		INDEX idx_name (name),
		INDEX idx_age (age)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_force_index")

	// Force use of specific index
	rows, err := db.Query("SELECT * FROM test_force_index FORCE INDEX (idx_name) WHERE name = 'test'")
	assert.NoError(t, err)
	if rows != nil {
		rows.Close()
	}
}

// TestMySQLSpecific_USE_INDEX tests USE INDEX hint
func TestMySQLSpecific_USE_INDEX(t *testing.T) {
	t.Skip("USE INDEX hint not supported by PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_use_index")
	db.Exec("CREATE TABLE test_use_index (id INT PRIMARY KEY, val INT, INDEX idx_val (val))")
	defer db.Exec("DROP TABLE IF EXISTS test_use_index")

	rows, err := db.Query("SELECT * FROM test_use_index USE INDEX (idx_val) WHERE val > 10")
	assert.NoError(t, err)
	if rows != nil {
		rows.Close()
	}
}

// TestMySQLSpecific_IGNORE_INDEX tests IGNORE INDEX hint
func TestMySQLSpecific_IGNORE_INDEX(t *testing.T) {
	t.Skip("IGNORE INDEX hint not supported by PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_ignore_index")
	db.Exec("CREATE TABLE test_ignore_index (id INT PRIMARY KEY, val INT, INDEX idx_val (val))")
	defer db.Exec("DROP TABLE IF EXISTS test_ignore_index")

	rows, err := db.Query("SELECT * FROM test_ignore_index IGNORE INDEX (idx_val) WHERE val > 10")
	assert.NoError(t, err)
	if rows != nil {
		rows.Close()
	}
}

// TestMySQLSpecific_INSERT_DELAYED tests INSERT DELAYED
// Deprecated in MySQL 5.6, removed in 5.7
func TestMySQLSpecific_INSERT_DELAYED(t *testing.T) {
	t.Skip("INSERT DELAYED deprecated in MySQL and not supported by PostgreSQL")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_delayed")
	db.Exec("CREATE TABLE test_delayed (id INT AUTO_INCREMENT PRIMARY KEY, val INT)")
	defer db.Exec("DROP TABLE IF EXISTS test_delayed")

	_, err = db.Exec("INSERT DELAYED INTO test_delayed (val) VALUES (100)")
	// Note: This will likely fail even in modern MySQL
	t.Log("INSERT DELAYED result:", err)
}

// TestMySQLSpecific_PARTITION_Syntax tests MySQL partition syntax
// PG has different partition syntax (declarative partitioning)
func TestMySQLSpecific_PARTITION_Syntax(t *testing.T) {
	t.Skip("MySQL PARTITION BY syntax not compatible with PostgreSQL declarative partitioning")

	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	db.Exec("DROP TABLE IF EXISTS test_partition")
	_, err = db.Exec(`CREATE TABLE test_partition (
		id INT,
		created_year INT
	) PARTITION BY RANGE (created_year) (
		PARTITION p2020 VALUES LESS THAN (2021),
		PARTITION p2021 VALUES LESS THAN (2022),
		PARTITION p2022 VALUES LESS THAN (2023)
	)`)
	assert.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_partition")
}
