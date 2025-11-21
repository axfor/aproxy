package integration

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	proxyDSN = "root:@tcp(localhost:3306)/test?parseTime=true"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func setupTestDB(tb testing.TB) (*sql.DB, func()) {
	db, err := sql.Open("mysql", proxyDSN)
	require.NoError(tb, err)

	err = db.Ping()
	require.NoError(tb, err)

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

func cleanupPostgreSQL(tb testing.TB, tableName string) {
	cleanupPostgreSQLWithDB(tb, nil, tableName)
}

func cleanupPostgreSQLWithDB(tb testing.TB, db *sql.DB, tableName string) {
	// If db is provided, use it; otherwise create a temporary one
	var ownDB *sql.DB
	var err error

	if db == nil {
		ownDB, err = sql.Open("mysql", proxyDSN)
		if err != nil {
			tb.Logf("Warning: failed to open cleanup connection: %v", err)
			return
		}
		defer ownDB.Close()
		db = ownDB
	}

	_, err = db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName))
	if err != nil {
		tb.Logf("Warning: failed to drop table %s: %v", tableName, err)
	}
}

func TestBasicQuery(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("SELECT 1", func(t *testing.T) {
		var result int
		err := db.QueryRow("SELECT 1").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, 1, result)
	})

	t.Run("SELECT NOW()", func(t *testing.T) {
		var nowStr string
		err := db.QueryRow("SELECT NOW()").Scan(&nowStr)
		assert.NoError(t, err)
		// Parse the string to time.Time for comparison
		// PostgreSQL's CURRENT_TIMESTAMP returns timestamp in the database's timezone
		// The aproxy server may be using UTC while the test is in local timezone
		var now time.Time
		formats := []string{
			time.RFC3339,           // PostgreSQL: 2006-01-02T15:04:05Z07:00
			time.RFC3339Nano,       // PostgreSQL with nanoseconds
			"2006-01-02 15:04:05",  // PostgreSQL/MySQL: 2006-01-02 15:04:05
		}
		parsed := false
		for _, format := range formats {
			// Try parsing with explicit timezone from string first (like RFC3339)
			now, err = time.Parse(format, nowStr)
			if err == nil {
				parsed = true
				break
			}
			// Try parsing as UTC (most common for PostgreSQL CURRENT_TIMESTAMP)
			now, err = time.ParseInLocation(format, nowStr, time.UTC)
			if err == nil {
				parsed = true
				break
			}
			// Finally try local timezone
			now, err = time.ParseInLocation(format, nowStr, time.Local)
			if err == nil {
				parsed = true
				break
			}
		}
		assert.True(t, parsed, "Should be able to parse NOW() result: "+nowStr)
		// Compare times using Unix timestamps (timezone-independent)
		// Allow up to 9 hours delta to handle timezone differences (CST+8, etc.)
		nowUTC := time.Now().UTC()
		nowParsedUTC := now.UTC()
		delta := float64(nowUTC.Unix()) - float64(nowParsedUTC.Unix())
		assert.True(t, delta >= -32400 && delta <= 32400, "NOW() should return current time (delta: %.0f seconds)", delta)
	})
}

func TestCreateTable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_users")

	t.Run("Create table with AUTO_INCREMENT", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_users (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(100),
				email VARCHAR(100),
				created_at TIMESTAMP DEFAULT NOW()
			)
		`)
		assert.NoError(t, err)
	})
}

func TestInsertAndSelect(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_products")

	_, err := db.Exec(`
		CREATE TABLE test_products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	t.Run("Insert single row", func(t *testing.T) {
		result, err := db.Exec("INSERT INTO test_products (name, price) VALUES (?, ?)", "Product 1", 99.99)
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)
	})

	t.Run("Select inserted data", func(t *testing.T) {
		var id int
		var name string
		var price float64

		err := db.QueryRow("SELECT id, name, price FROM test_products WHERE name = ?", "Product 1").Scan(&id, &name, &price)
		assert.NoError(t, err)
		assert.Equal(t, "Product 1", name)
		assert.Equal(t, 99.99, price)
	})
}

func TestPreparedStatements(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_items")

	_, err := db.Exec(`
		CREATE TABLE test_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			quantity INT
		)
	`)
	require.NoError(t, err)

	t.Run("Prepare and execute", func(t *testing.T) {
		stmt, err := db.Prepare("INSERT INTO test_items (name, quantity) VALUES (?, ?)")
		require.NoError(t, err)
		defer stmt.Close()

		_, err = stmt.Exec("Item 1", 10)
		assert.NoError(t, err)

		_, err = stmt.Exec("Item 2", 20)
		assert.NoError(t, err)

		_, err = stmt.Exec("Item 3", 30)
		assert.NoError(t, err)
	})

	t.Run("Verify inserted data", func(t *testing.T) {
		rows, err := db.Query("SELECT name, quantity FROM test_items ORDER BY id")
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var name string
			var quantity int
			err := rows.Scan(&name, &quantity)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 3, count)
	})
}

func TestTransaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_accounts")

	_, err := db.Exec(`
		CREATE TABLE test_accounts (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			balance DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	t.Run("Commit transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO test_accounts (name, balance) VALUES (?, ?)", "Alice", 1000.00)
		assert.NoError(t, err)

		_, err = tx.Exec("INSERT INTO test_accounts (name, balance) VALUES (?, ?)", "Bob", 2000.00)
		assert.NoError(t, err)

		err = tx.Commit()
		assert.NoError(t, err)

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM test_accounts").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})

	t.Run("Rollback transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO test_accounts (name, balance) VALUES (?, ?)", "Charlie", 3000.00)
		assert.NoError(t, err)

		err = tx.Rollback()
		assert.NoError(t, err)

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM test_accounts").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestShowCommands(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("SHOW DATABASES", func(t *testing.T) {
		rows, err := db.Query("SHOW DATABASES")
		require.NoError(t, err)
		defer rows.Close()

		found := false
		for rows.Next() {
			var dbName string
			err := rows.Scan(&dbName)
			assert.NoError(t, err)
			if dbName != "" {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("SHOW TABLES", func(t *testing.T) {
		rows, err := db.Query("SHOW TABLES")
		require.NoError(t, err)
		defer rows.Close()

		for rows.Next() {
			var tableName string
			err := rows.Scan(&tableName)
			assert.NoError(t, err)
		}
	})
}

func TestUpdateAndDelete(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_records")

	_, err := db.Exec(`
		CREATE TABLE test_records (
			id INT AUTO_INCREMENT PRIMARY KEY,
			value VARCHAR(100)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO test_records (value) VALUES (?), (?), (?)", "A", "B", "C")
	require.NoError(t, err)

	t.Run("Update row", func(t *testing.T) {
		result, err := db.Exec("UPDATE test_records SET value = ? WHERE value = ?", "D", "B")
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)
	})

	t.Run("Delete row", func(t *testing.T) {
		result, err := db.Exec("DELETE FROM test_records WHERE value = ?", "A")
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)
	})

	t.Run("Verify final count", func(t *testing.T) {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM test_records").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count)
	})
}

func TestConcurrentConnections(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	t.Run("Multiple concurrent queries", func(t *testing.T) {
		done := make(chan bool, 10)

		for i := 0; i < 10; i++ {
			go func(id int) {
				var result int
				err := db.QueryRow("SELECT ?", id).Scan(&result)
				assert.NoError(t, err)
				assert.Equal(t, id, result)
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// ============================================================================
// Comprehensive MySQL to PostgreSQL Compatibility Integration Tests
// Based on docs/MYSQL_TO_PG_CASES.md
// ============================================================================

func TestDataTypes_Integer(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_integers")

	t.Run("Create table with integer types", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_integers (
				id INT AUTO_INCREMENT PRIMARY KEY,
				tiny_val TINYINT,
				small_val SMALLINT,
				medium_val INT,
				big_val BIGINT,
				unsigned_val INT UNSIGNED
			)
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert integer values", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO test_integers (tiny_val, small_val, medium_val, big_val, unsigned_val)
			VALUES (?, ?, ?, ?, ?)
		`, 127, 32767, 2147483647, 9223372036854775807, 4294967295)
		assert.NoError(t, err)
	})

	t.Run("Select and verify integer values", func(t *testing.T) {
		var id, tinyVal, smallVal, mediumVal int64
		var bigVal uint64
		var unsignedVal uint32

		err := db.QueryRow(`
			SELECT id, tiny_val, small_val, medium_val, big_val, unsigned_val
			FROM test_integers LIMIT 1
		`).Scan(&id, &tinyVal, &smallVal, &mediumVal, &bigVal, &unsignedVal)

		assert.NoError(t, err)
		assert.Greater(t, id, int64(0))
		assert.Equal(t, int64(127), tinyVal)
		assert.Equal(t, int64(32767), smallVal)
		assert.Equal(t, int64(2147483647), mediumVal)
	})
}

func TestDataTypes_FloatingPoint(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_floats")

	t.Run("Create table with floating point types", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_floats (
				id INT AUTO_INCREMENT PRIMARY KEY,
				float_val FLOAT,
				double_val DOUBLE,
				decimal_val DECIMAL(10, 2)
			)
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert and verify floating point values", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO test_floats (float_val, double_val, decimal_val)
			VALUES (?, ?, ?)
		`, 3.14, 3.141592653589793, 12345.67)
		assert.NoError(t, err)

		var floatVal, doubleVal, decimalVal float64
		err = db.QueryRow(`
			SELECT float_val, double_val, decimal_val FROM test_floats LIMIT 1
		`).Scan(&floatVal, &doubleVal, &decimalVal)

		assert.NoError(t, err)
		assert.InDelta(t, 3.14, floatVal, 0.01)
		assert.InDelta(t, 3.141592653589793, doubleVal, 0.000001)
		assert.Equal(t, 12345.67, decimalVal)
	})
}

func TestDataTypes_String(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_strings")

	t.Run("Create table with string types", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_strings (
				id INT AUTO_INCREMENT PRIMARY KEY,
				varchar_val VARCHAR(255),
				char_val CHAR(10),
				text_val TEXT,
				tinytext_val TINYTEXT,
				mediumtext_val MEDIUMTEXT,
				longtext_val LONGTEXT
			)
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert and verify string values", func(t *testing.T) {
		testStr := "Hello, World!"
		longStr := "This is a long text content for testing text types in the database"

		_, err := db.Exec(`
			INSERT INTO test_strings (varchar_val, char_val, text_val, tinytext_val, mediumtext_val, longtext_val)
			VALUES (?, ?, ?, ?, ?, ?)
		`, testStr, "fixed", longStr, testStr, longStr, longStr)
		assert.NoError(t, err)

		var varcharVal, charVal, textVal, tinytextVal, mediumtextVal, longtextVal string
		err = db.QueryRow(`
			SELECT varchar_val, char_val, text_val, tinytext_val, mediumtext_val, longtext_val
			FROM test_strings LIMIT 1
		`).Scan(&varcharVal, &charVal, &textVal, &tinytextVal, &mediumtextVal, &longtextVal)

		assert.NoError(t, err)
		assert.Equal(t, testStr, varcharVal)
		assert.Contains(t, charVal, "fixed")
		assert.Equal(t, longStr, textVal)
	})
}

func TestDataTypes_DateTime(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_datetime")

	t.Run("Create table with datetime types", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_datetime (
				id INT AUTO_INCREMENT PRIMARY KEY,
				date_val DATE,
				time_val TIME,
				datetime_val DATETIME,
				timestamp_val TIMESTAMP,
				created_at DATETIME DEFAULT NOW()
			)
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert and verify datetime values", func(t *testing.T) {
		testDate := time.Date(2024, 12, 25, 15, 30, 45, 0, time.UTC)

		_, err := db.Exec(`
			INSERT INTO test_datetime (date_val, time_val, datetime_val, timestamp_val)
			VALUES (?, ?, ?, ?)
		`, testDate, testDate, testDate, testDate)
		assert.NoError(t, err)

		// Note: TIME and DATETIME types are scanned as strings
		// This is due to how aproxy formats datetime values in Text Protocol
		var dateVal time.Time
		var timeVal, datetimeVal, timestampVal, createdAt string
		err = db.QueryRow(`
			SELECT date_val, time_val, datetime_val, timestamp_val, created_at
			FROM test_datetime LIMIT 1
		`).Scan(&dateVal, &timeVal, &datetimeVal, &timestampVal, &createdAt)

		assert.NoError(t, err)
		assert.Equal(t, testDate.Year(), dateVal.Year())
		assert.Equal(t, testDate.Month(), dateVal.Month())
		assert.Equal(t, testDate.Day(), dateVal.Day())
		assert.Equal(t, "15:30:45", timeVal) // TIME values are returned as "HH:MM:SS" strings

		// Parse datetime strings to verify they're close to insertion time
		// Try MySQL format first, then ISO 8601 format
		datetimeParsed, err := time.Parse("2006-01-02 15:04:05", datetimeVal)
		if err != nil {
			// Try ISO 8601 format (with timezone)
			datetimeParsed, err = time.Parse(time.RFC3339, datetimeVal)
			if err != nil {
				// Try ISO 8601 without timezone
				datetimeParsed, err = time.Parse("2006-01-02T15:04:05", datetimeVal)
			}
		}
		assert.NoError(t, err, "Failed to parse datetime value: %s", datetimeVal)
		assert.Equal(t, 2024, datetimeParsed.Year())
		assert.Equal(t, time.December, datetimeParsed.Month())
		assert.Equal(t, 25, datetimeParsed.Day())

		// Verify created_at is close to current time
		// Try MySQL format first, then ISO 8601 format
		createdAtParsed, err := time.Parse("2006-01-02 15:04:05", createdAt)
		var hasTimezone bool
		if err != nil {
			// Try ISO 8601 format (with timezone)
			createdAtParsed, err = time.Parse(time.RFC3339, createdAt)
			if err == nil {
				hasTimezone = true
			} else {
				// Try ISO 8601 without timezone
				createdAtParsed, err = time.Parse("2006-01-02T15:04:05", createdAt)
			}
		}
		assert.NoError(t, err, "Failed to parse created_at value: %s", createdAt)
		// If parsed time has no timezone, assume UTC (PostgreSQL default)
		if !hasTimezone && createdAtParsed.Location() != time.UTC {
			createdAtParsed = time.Date(createdAtParsed.Year(), createdAtParsed.Month(), createdAtParsed.Day(),
				createdAtParsed.Hour(), createdAtParsed.Minute(), createdAtParsed.Second(), createdAtParsed.Nanosecond(), time.UTC)
		}
		// Allow up to 24 hours delta to handle timezone differences
		nowUTC := time.Now().UTC()
		parsedUTC := createdAtParsed.UTC()
		delta := nowUTC.Sub(parsedUTC)
		absDelta := delta
		if absDelta < 0 {
			absDelta = -absDelta
		}
		assert.True(t, absDelta <= 24*time.Hour, "created_at should be close to current time (delta: %v)", delta)
	})
}

func TestFunctions_DateTime(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name  string
		query string
	}{
		{"NOW()", "SELECT NOW()"},
		{"CURDATE()", "SELECT CURDATE()"},
		{"CURTIME()", "SELECT CURTIME()"},
		{"CURRENT_TIMESTAMP", "SELECT CURRENT_TIMESTAMP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			err := db.QueryRow(tt.query).Scan(&result)
			assert.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestFunctions_String(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name     string
		query    string
		expected interface{}
	}{
		{"CONCAT", "SELECT CONCAT('Hello', ' ', 'World')", "Hello World"},
		{"UPPER", "SELECT UPPER('hello')", "HELLO"},
		{"LOWER", "SELECT LOWER('WORLD')", "world"},
		{"TRIM", "SELECT TRIM('  test  ')", "test"},
		{"LENGTH", "SELECT CHAR_LENGTH('hello')", int64(5)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result interface{}
			err := db.QueryRow(tt.query).Scan(&result)
			assert.NoError(t, err)

			switch expected := tt.expected.(type) {
			case string:
				assert.Equal(t, expected, string(result.([]byte)))
			case int64:
				// Handle case where PostgreSQL returns numeric result as string/bytes
				switch v := result.(type) {
				case int64:
					assert.Equal(t, expected, v)
				case []byte:
					// Convert bytes to string and parse as int
					val, err := strconv.ParseInt(string(v), 10, 64)
					assert.NoError(t, err)
					assert.Equal(t, expected, val)
				default:
					t.Fatalf("Unexpected result type: %T", result)
				}
			}
		})
	}
}

func TestFunctions_Aggregate(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_aggregates")

	_, err := db.Exec(`
		CREATE TABLE test_aggregates (
			id INT AUTO_INCREMENT PRIMARY KEY,
			category VARCHAR(50),
			value INT
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO test_aggregates (category, value) VALUES
		('A', 10), ('A', 20), ('B', 30), ('B', 40), ('C', 50)
	`)
	require.NoError(t, err)

	t.Run("COUNT", func(t *testing.T) {
		var count int64
		err := db.QueryRow("SELECT COUNT(*) FROM test_aggregates").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})

	t.Run("SUM", func(t *testing.T) {
		var sum int64
		err := db.QueryRow("SELECT SUM(value) FROM test_aggregates").Scan(&sum)
		assert.NoError(t, err)
		assert.Equal(t, int64(150), sum)
	})

	t.Run("AVG", func(t *testing.T) {
		var avg float64
		err := db.QueryRow("SELECT AVG(value) FROM test_aggregates").Scan(&avg)
		assert.NoError(t, err)
		assert.Equal(t, 30.0, avg)
	})

	t.Run("MAX", func(t *testing.T) {
		var max int64
		err := db.QueryRow("SELECT MAX(value) FROM test_aggregates").Scan(&max)
		assert.NoError(t, err)
		assert.Equal(t, int64(50), max)
	})

	t.Run("MIN", func(t *testing.T) {
		var min int64
		err := db.QueryRow("SELECT MIN(value) FROM test_aggregates").Scan(&min)
		assert.NoError(t, err)
		assert.Equal(t, int64(10), min)
	})
}

func TestComplexQueries_Joins(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_users_join")
	defer cleanupPostgreSQL(t, "test_orders_join")

	_, err := db.Exec(`
		CREATE TABLE test_users_join (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE test_orders_join (
			id INT AUTO_INCREMENT PRIMARY KEY,
			user_id INT,
			total DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO test_users_join (name) VALUES ('Alice'), ('Bob'), ('Charlie')
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO test_orders_join (user_id, total) VALUES
		(1, 100.00), (1, 200.00), (2, 300.00)
	`)
	require.NoError(t, err)

	t.Run("INNER JOIN", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT u.name, o.total
			FROM test_users_join u
			INNER JOIN test_orders_join o ON u.id = o.user_id
		`)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var name string
			var total float64
			err := rows.Scan(&name, &total)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 3, count)
	})

	t.Run("LEFT JOIN", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT u.name, o.total
			FROM test_users_join u
			LEFT JOIN test_orders_join o ON u.id = o.user_id
			ORDER BY u.id
		`)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var name string
			var total sql.NullFloat64
			err := rows.Scan(&name, &total)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 4, count) // 3 orders + 1 user without orders
	})
}

func TestComplexQueries_Subqueries(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_products_sub")

	_, err := db.Exec(`
		CREATE TABLE test_products_sub (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO test_products_sub (name, price) VALUES
		('Product A', 10.00),
		('Product B', 20.00),
		('Product C', 30.00),
		('Product D', 40.00)
	`)
	require.NoError(t, err)

	t.Run("Subquery with IN", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT name FROM test_products_sub
			WHERE id IN (SELECT id FROM test_products_sub WHERE price > ?)
		`, 15.00)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var name string
			err := rows.Scan(&name)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 3, count) // B, C, D
	})

	t.Run("Subquery in SELECT", func(t *testing.T) {
		var avgPrice float64
		err := db.QueryRow(`
			SELECT AVG(price) FROM test_products_sub WHERE price < (SELECT MAX(price) FROM test_products_sub)
		`).Scan(&avgPrice)
		assert.NoError(t, err)
		assert.Equal(t, 20.0, avgPrice)
	})
}

func TestComplexQueries_GroupBy(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_sales")

	_, err := db.Exec(`
		CREATE TABLE test_sales (
			id INT AUTO_INCREMENT PRIMARY KEY,
			category VARCHAR(50),
			amount DECIMAL(10, 2)
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO test_sales (category, amount) VALUES
		('Electronics', 100.00),
		('Electronics', 200.00),
		('Books', 50.00),
		('Books', 75.00),
		('Clothing', 150.00)
	`)
	require.NoError(t, err)

	t.Run("GROUP BY with aggregates", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT category, COUNT(*) as count, SUM(amount) as total
			FROM test_sales
			GROUP BY category
			ORDER BY total DESC
		`)
		require.NoError(t, err)
		defer rows.Close()

		results := make(map[string]float64)
		for rows.Next() {
			var category string
			var count int
			var total float64
			err := rows.Scan(&category, &count, &total)
			assert.NoError(t, err)
			results[category] = total
		}

		assert.Equal(t, 300.0, results["Electronics"])
		assert.Equal(t, 125.0, results["Books"])
		assert.Equal(t, 150.0, results["Clothing"])
	})

	t.Run("GROUP BY with HAVING", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT category, SUM(amount) as total
			FROM test_sales
			GROUP BY category
			HAVING SUM(amount) > ?
		`, 100.00)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var category string
			var total float64
			err := rows.Scan(&category, &total)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 3, count)
	})
}

func TestLimitOffset(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_pagination")

	_, err := db.Exec(`
		CREATE TABLE test_pagination (
			id INT AUTO_INCREMENT PRIMARY KEY,
			value INT
		)
	`)
	require.NoError(t, err)

	for i := 1; i <= 20; i++ {
		_, err := db.Exec("INSERT INTO test_pagination (value) VALUES (?)", i)
		require.NoError(t, err)
	}

	t.Run("LIMIT only", func(t *testing.T) {
		rows, err := db.Query("SELECT value FROM test_pagination LIMIT 5")
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			count++
		}
		assert.Equal(t, 5, count)
	})

	t.Run("LIMIT with OFFSET (MySQL syntax)", func(t *testing.T) {
		rows, err := db.Query("SELECT value FROM test_pagination LIMIT 5, 10")
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		firstValue := 0
		for rows.Next() {
			var value int
			err := rows.Scan(&value)
			assert.NoError(t, err)
			if count == 0 {
				firstValue = value
			}
			count++
		}
		assert.Equal(t, 10, count)
		assert.Equal(t, 6, firstValue) // Should start from 6th row (offset 5)
	})
}

func TestNullValues(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_nulls")

	_, err := db.Exec(`
		CREATE TABLE test_nulls (
			id INT AUTO_INCREMENT PRIMARY KEY,
			nullable_int INT,
			nullable_string VARCHAR(100),
			nullable_date DATETIME
		)
	`)
	require.NoError(t, err)

	t.Run("Insert NULL values", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO test_nulls (nullable_int, nullable_string, nullable_date)
			VALUES (NULL, NULL, NULL)
		`)
		assert.NoError(t, err)
	})

	t.Run("Query NULL values", func(t *testing.T) {
		var nullInt sql.NullInt64
		var nullString sql.NullString
		var nullDate sql.NullTime

		err := db.QueryRow(`
			SELECT nullable_int, nullable_string, nullable_date
			FROM test_nulls LIMIT 1
		`).Scan(&nullInt, &nullString, &nullDate)

		assert.NoError(t, err)
		assert.False(t, nullInt.Valid)
		assert.False(t, nullString.Valid)
		assert.False(t, nullDate.Valid)
	})

	t.Run("IFNULL function", func(t *testing.T) {
		var result string
		err := db.QueryRow("SELECT IFNULL(nullable_string, 'default') FROM test_nulls LIMIT 1").Scan(&result)
		assert.NoError(t, err)
		assert.Equal(t, "default", result)
	})
}

func TestBatchOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_batch")

	_, err := db.Exec(`
		CREATE TABLE test_batch (
			id INT AUTO_INCREMENT PRIMARY KEY,
			value VARCHAR(100)
		)
	`)
	require.NoError(t, err)

	t.Run("Batch insert", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO test_batch (value) VALUES
			('A'), ('B'), ('C'), ('D'), ('E'),
			('F'), ('G'), ('H'), ('I'), ('J')
		`)
		assert.NoError(t, err)

		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM test_batch").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 10, count)
	})

	t.Run("Batch update", func(t *testing.T) {
		result, err := db.Exec("UPDATE test_batch SET value = CONCAT(value, '_updated') WHERE id <= 5")
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(5), affected)
	})

	t.Run("Batch delete", func(t *testing.T) {
		result, err := db.Exec("DELETE FROM test_batch WHERE id > 8")
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(2), affected)
	})
}

func TestIndexes(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "test_indexes")

	t.Run("Create table with indexes", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_indexes (
				id INT AUTO_INCREMENT PRIMARY KEY,
				email VARCHAR(255) UNIQUE,
				name VARCHAR(100),
				age INT,
				INDEX idx_name (name),
				INDEX idx_age_name (age, name)
			)
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert and query with indexes", func(t *testing.T) {
		_, err := db.Exec(`
			INSERT INTO test_indexes (email, name, age) VALUES
			('alice@example.com', 'Alice', 30),
			('bob@example.com', 'Bob', 25),
			('charlie@example.com', 'Charlie', 35)
		`)
		assert.NoError(t, err)

		var name string
		err = db.QueryRow("SELECT name FROM test_indexes WHERE email = ?", "bob@example.com").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "Bob", name)
	})

	t.Run("Unique constraint violation", func(t *testing.T) {
		_, err := db.Exec("INSERT INTO test_indexes (email, name, age) VALUES (?, ?, ?)", "alice@example.com", "Alice2", 31)
		assert.Error(t, err) // Should fail due to unique constraint
	})
}

func BenchmarkSimpleQuery(b *testing.B) {
	db, cleanup := setupTestDB(b)
	defer cleanup()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result int
		db.QueryRow("SELECT 1").Scan(&result)
	}
}

func BenchmarkPreparedStatement(b *testing.B) {
	db, cleanup := setupTestDB(b)
	defer cleanup()

	stmt, _ := db.Prepare("SELECT ?")
	defer stmt.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result int
		stmt.QueryRow(i).Scan(&result)
	}
}

func BenchmarkComplexQuery(b *testing.B) {
	db, cleanup := setupTestDB(b)
	defer cleanup()

	db.Exec(`CREATE TABLE bench_test (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(100), value INT)`)
	defer cleanupPostgreSQL(b, "bench_test")

	for i := 0; i < 100; i++ {
		db.Exec("INSERT INTO bench_test (name, value) VALUES (?, ?)", fmt.Sprintf("name_%d", i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, _ := db.Query(`
			SELECT name, SUM(value) as total
			FROM bench_test
			WHERE value > ?
			GROUP BY name
			HAVING SUM(value) > ?
			LIMIT 10
		`, 10, 20)
		if rows != nil {
			rows.Close()
		}
	}
}
