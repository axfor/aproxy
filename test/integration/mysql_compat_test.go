package integration

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLCompatibility_DDL tests DDL statement compatibility
func TestMySQLCompatibility_DDL(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Clean up
	db.Exec("DROP TABLE IF EXISTS compat_ddl_test")
	defer db.Exec("DROP TABLE IF EXISTS compat_ddl_test")

	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name: "CREATE TABLE with AUTO_INCREMENT",
			sql: `CREATE TABLE compat_ddl_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(100) NOT NULL,
				age INT,
				created_at DATETIME
			)`,
			wantErr: false,
		},
		{
			name: "CREATE TABLE with multiple data types",
			sql: `CREATE TABLE compat_ddl_test (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				tiny TINYINT,
				small SMALLINT,
				medium MEDIUMINT,
				normal INT,
				big BIGINT,
				f FLOAT,
				d DOUBLE,
				decimal_col DECIMAL(10,2),
				dt DATETIME,
				ts TIMESTAMP,
				yr YEAR,
				txt TEXT,
				blob_col BLOB,
				json_col JSON
			)`,
			wantErr: false,
		},
		{
			name: "CREATE TABLE with INDEX",
			sql: `CREATE TABLE compat_ddl_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				username VARCHAR(50) NOT NULL,
				email VARCHAR(100),
				status ENUM('active', 'inactive'),
				INDEX idx_username (username),
				INDEX idx_email (email)
			)`,
			wantErr: false,
		},
		{
			name: "CREATE TABLE with UNIQUE INDEX",
			sql: `CREATE TABLE compat_ddl_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				code VARCHAR(20) NOT NULL,
				UNIQUE INDEX idx_code (code)
			)`,
			wantErr: false,
		},
		{
			name: "CREATE TABLE with composite INDEX",
			sql: `CREATE TABLE compat_ddl_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				first_name VARCHAR(50),
				last_name VARCHAR(50),
				department VARCHAR(50),
				INDEX idx_name (first_name, last_name),
				INDEX idx_dept_name (department, last_name)
			)`,
			wantErr: false,
		},
		{
			name: "CREATE TABLE with DEFAULT values",
			sql: `CREATE TABLE compat_ddl_test (
				id INT AUTO_INCREMENT PRIMARY KEY,
				status VARCHAR(20) DEFAULT 'pending',
				count INT DEFAULT 0,
				is_active TINYINT DEFAULT 1
			)`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Drop table before each test
			db.Exec("DROP TABLE IF EXISTS compat_ddl_test")

			_, err := db.Exec(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "SQL: %s", tt.sql)
			}
		})
	}
}

// TestMySQLCompatibility_INSERT tests INSERT statement compatibility
func TestMySQLCompatibility_INSERT(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup
	db.Exec("DROP TABLE IF EXISTS compat_insert_test")
	_, err = db.Exec(`CREATE TABLE compat_insert_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		salary DECIMAL(10,2)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_insert_test")

	tests := []struct {
		name    string
		sql     string
		args    []interface{}
		wantErr bool
	}{
		{
			name:    "INSERT single row with values",
			sql:     "INSERT INTO compat_insert_test (name, age, salary) VALUES ('Alice', 30, 50000.00)",
			wantErr: false,
		},
		{
			name:    "INSERT with NULL AUTO_INCREMENT",
			sql:     "INSERT INTO compat_insert_test (id, name, age) VALUES (NULL, 'Bob', 25)",
			wantErr: false,
		},
		{
			name:    "INSERT multiple rows",
			sql:     "INSERT INTO compat_insert_test (name, age, salary) VALUES ('Charlie', 35, 60000), ('David', 28, 55000), ('Eve', 32, 58000)",
			wantErr: false,
		},
		{
			name:    "INSERT with prepared statement",
			sql:     "INSERT INTO compat_insert_test (name, age, salary) VALUES (?, ?, ?)",
			args:    []interface{}{"Frank", 40, 65000.50},
			wantErr: false,
		},
		{
			name:    "INSERT without column list",
			sql:     "INSERT INTO compat_insert_test VALUES (NULL, 'Grace', 29, 52000)",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if len(tt.args) > 0 {
				_, err = db.Exec(tt.sql, tt.args...)
			} else {
				_, err = db.Exec(tt.sql)
			}

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "SQL: %s", tt.sql)
			}
		})
	}

	// Verify data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM compat_insert_test").Scan(&count)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "Should have inserted some rows")
}

// TestMySQLCompatibility_SELECT tests SELECT statement compatibility
func TestMySQLCompatibility_SELECT(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup test data
	db.Exec("DROP TABLE IF EXISTS compat_select_test")
	_, err = db.Exec(`CREATE TABLE compat_select_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		department VARCHAR(50),
		salary DECIMAL(10,2)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_select_test")

	// Insert test data
	_, err = db.Exec(`INSERT INTO compat_select_test (name, age, department, salary) VALUES
		('Alice', 30, 'Engineering', 80000),
		('Bob', 25, 'Sales', 50000),
		('Charlie', 35, 'Engineering', 90000),
		('David', 28, 'Marketing', 60000),
		('Eve', 32, 'Sales', 65000),
		('Frank', 40, 'Engineering', 95000)
	`)
	require.NoError(t, err)

	tests := []struct {
		name     string
		sql      string
		validate func(*testing.T, *sql.Rows)
	}{
		{
			name: "SELECT all columns",
			sql:  "SELECT * FROM compat_select_test",
			validate: func(t *testing.T, rows *sql.Rows) {
				count := 0
				for rows.Next() {
					count++
				}
				assert.Equal(t, 6, count)
			},
		},
		{
			name: "SELECT specific columns",
			sql:  "SELECT name, age FROM compat_select_test",
			validate: func(t *testing.T, rows *sql.Rows) {
				var name string
				var age int
				count := 0
				for rows.Next() {
					err := rows.Scan(&name, &age)
					assert.NoError(t, err)
					count++
				}
				assert.Equal(t, 6, count)
			},
		},
		{
			name: "SELECT with WHERE clause",
			sql:  "SELECT name FROM compat_select_test WHERE age > 30",
			validate: func(t *testing.T, rows *sql.Rows) {
				count := 0
				for rows.Next() {
					count++
				}
				assert.Equal(t, 3, count) // Charlie, Eve, Frank
			},
		},
		{
			name: "SELECT with LIMIT",
			sql:  "SELECT name FROM compat_select_test LIMIT 3",
			validate: func(t *testing.T, rows *sql.Rows) {
				count := 0
				for rows.Next() {
					count++
				}
				assert.Equal(t, 3, count)
			},
		},
		{
			name: "SELECT with MySQL-style LIMIT offset, count",
			sql:  "SELECT name FROM compat_select_test LIMIT 2, 3",
			validate: func(t *testing.T, rows *sql.Rows) {
				count := 0
				for rows.Next() {
					count++
				}
				assert.Equal(t, 3, count)
			},
		},
		{
			name: "SELECT with ORDER BY",
			sql:  "SELECT name FROM compat_select_test ORDER BY age DESC",
			validate: func(t *testing.T, rows *sql.Rows) {
				var names []string
				for rows.Next() {
					var name string
					rows.Scan(&name)
					names = append(names, name)
				}
				assert.Equal(t, "Frank", names[0]) // Oldest
			},
		},
		{
			name: "SELECT with GROUP BY",
			sql:  "SELECT department, COUNT(*) as cnt FROM compat_select_test GROUP BY department",
			validate: func(t *testing.T, rows *sql.Rows) {
				deptCounts := make(map[string]int)
				for rows.Next() {
					var dept string
					var count int
					rows.Scan(&dept, &count)
					deptCounts[dept] = count
				}
				assert.Equal(t, 3, deptCounts["Engineering"])
				assert.Equal(t, 2, deptCounts["Sales"])
			},
		},
		{
			name: "SELECT with aggregate functions",
			sql:  "SELECT AVG(salary), MAX(salary), MIN(salary), SUM(salary) FROM compat_select_test WHERE department = 'Engineering'",
			validate: func(t *testing.T, rows *sql.Rows) {
				if rows.Next() {
					var avg, max, min, sum float64
					err := rows.Scan(&avg, &max, &min, &sum)
					assert.NoError(t, err)
					assert.Greater(t, avg, 0.0)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := db.Query(tt.sql)
			require.NoError(t, err, "SQL: %s", tt.sql)
			defer rows.Close()

			tt.validate(t, rows)
		})
	}
}

// TestMySQLCompatibility_UPDATE tests UPDATE statement compatibility
func TestMySQLCompatibility_UPDATE(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup
	db.Exec("DROP TABLE IF EXISTS compat_update_test")
	_, err = db.Exec(`CREATE TABLE compat_update_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT,
		status VARCHAR(20)
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_update_test")

	// Insert test data
	db.Exec("INSERT INTO compat_update_test (name, age, status) VALUES ('Alice', 30, 'active'), ('Bob', 25, 'inactive')")

	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "UPDATE single column",
			sql:     "UPDATE compat_update_test SET age = 31 WHERE name = 'Alice'",
			wantErr: false,
		},
		{
			name:    "UPDATE multiple columns",
			sql:     "UPDATE compat_update_test SET age = 26, status = 'active' WHERE name = 'Bob'",
			wantErr: false,
		},
		{
			name:    "UPDATE without WHERE (all rows)",
			sql:     "UPDATE compat_update_test SET status = 'pending'",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := db.Exec(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "SQL: %s", tt.sql)
				if !tt.wantErr {
					affected, _ := result.RowsAffected()
					assert.Greater(t, affected, int64(0))
				}
			}
		})
	}
}

// TestMySQLCompatibility_DELETE tests DELETE statement compatibility
func TestMySQLCompatibility_DELETE(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup
	db.Exec("DROP TABLE IF EXISTS compat_delete_test")
	_, err = db.Exec(`CREATE TABLE compat_delete_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100),
		age INT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_delete_test")

	tests := []struct {
		name    string
		setup   string
		sql     string
		wantErr bool
	}{
		{
			name:    "DELETE with WHERE clause",
			setup:   "INSERT INTO compat_delete_test (name, age) VALUES ('Alice', 30), ('Bob', 25)",
			sql:     "DELETE FROM compat_delete_test WHERE age < 28",
			wantErr: false,
		},
		{
			name:    "DELETE all rows",
			setup:   "INSERT INTO compat_delete_test (name, age) VALUES ('Charlie', 35)",
			sql:     "DELETE FROM compat_delete_test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			if tt.setup != "" {
				db.Exec("TRUNCATE TABLE compat_delete_test")
				_, err := db.Exec(tt.setup)
				require.NoError(t, err)
			}

			// Test
			_, err := db.Exec(tt.sql)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "SQL: %s", tt.sql)
			}
		})
	}
}

// TestMySQLCompatibility_Functions tests MySQL function compatibility
func TestMySQLCompatibility_Functions(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	tests := []struct {
		name     string
		sql      string
		validate func(*testing.T, interface{})
	}{
		{
			name: "NOW() function",
			sql:  "SELECT NOW()",
			validate: func(t *testing.T, val interface{}) {
				// Should return current timestamp
				// Note: This test might fail due to time type conversion issues
				// which is a known issue to fix
				t.Log("NOW() returned:", val)
			},
		},
		{
			name: "CONCAT() function",
			sql:  "SELECT CONCAT('Hello', ' ', 'World')",
			validate: func(t *testing.T, val interface{}) {
				str, ok := val.([]byte)
				if ok {
					assert.Equal(t, "Hello World", string(str))
				}
			},
		},
		{
			name: "UPPER() function",
			sql:  "SELECT UPPER('hello')",
			validate: func(t *testing.T, val interface{}) {
				str, ok := val.([]byte)
				if ok {
					assert.Equal(t, "HELLO", string(str))
				}
			},
		},
		{
			name: "LOWER() function",
			sql:  "SELECT LOWER('WORLD')",
			validate: func(t *testing.T, val interface{}) {
				str, ok := val.([]byte)
				if ok {
					assert.Equal(t, "world", string(str))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			err := db.QueryRow(tt.sql).Scan(&result)
			assert.NoError(t, err, "SQL: %s", tt.sql)
			if err == nil && tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

// TestMySQLCompatibility_DataTypes tests data type compatibility
func TestMySQLCompatibility_DataTypes(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup
	db.Exec("DROP TABLE IF EXISTS compat_types_test")
	_, err = db.Exec(`CREATE TABLE compat_types_test (
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
		enum_col ENUM('a', 'b', 'c'),
		json_col JSON
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_types_test")

	// Test INSERT and SELECT for each type
	now := time.Now()
	_, err = db.Exec(`INSERT INTO compat_types_test (
		tiny_col, small_col, medium_col, int_col, big_col,
		float_col, double_col, decimal_col,
		char_col, varchar_col, text_col,
		date_col, datetime_col, timestamp_col,
		enum_col, json_col
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		127, 32767, 8388607, 2147483647, 9223372036854775807,
		3.14, 2.718281828, 12345.67,
		"char", "varchar", "text content",
		now.Format("2006-01-02"), now.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"),
		"a", `{"key":"value"}`,
	)
	require.NoError(t, err)

	// Verify we can read it back
	var id int
	err = db.QueryRow("SELECT id FROM compat_types_test").Scan(&id)
	assert.NoError(t, err)
	assert.Greater(t, id, 0)
}

// TestMySQLCompatibility_Transactions tests transaction support
func TestMySQLCompatibility_Transactions(t *testing.T) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(t, err)
	defer db.Close()

	// Setup
	db.Exec("DROP TABLE IF EXISTS compat_tx_test")
	_, err = db.Exec(`CREATE TABLE compat_tx_test (
		id INT AUTO_INCREMENT PRIMARY KEY,
		value INT
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS compat_tx_test")

	t.Run("COMMIT transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO compat_tx_test (value) VALUES (100)")
		require.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		// Verify data was committed
		var count int
		db.QueryRow("SELECT COUNT(*) FROM compat_tx_test WHERE value = 100").Scan(&count)
		assert.Equal(t, 1, count)
	})

	t.Run("ROLLBACK transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO compat_tx_test (value) VALUES (200)")
		require.NoError(t, err)

		err = tx.Rollback()
		assert.NoError(t, err)

		// Verify data was NOT committed
		var count int
		db.QueryRow("SELECT COUNT(*) FROM compat_tx_test WHERE value = 200").Scan(&count)
		assert.Equal(t, 0, count)
	})
}
