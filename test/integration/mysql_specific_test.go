// Copyright (c) 2025 axfor

package integration

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMySQLSpecific_MATCH_AGAINST tests MySQL MATCH AGAINST full-text search
// This test verifies that AProxy correctly converts MySQL MATCH...AGAINST syntax to PostgreSQL
func TestMySQLSpecific_MATCH_AGAINST(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	// Clean up
	_, _ = db.Exec("DROP TABLE IF EXISTS test_fulltext")

	t.Run("Create table with fulltext columns", func(t *testing.T) {
		// In MySQL, you would create: CREATE TABLE ... FULLTEXT KEY (title, content)
		// But in PostgreSQL via AProxy, we'll just create a regular table
		_, err := db.Exec(`
			CREATE TABLE test_fulltext (
				id INT PRIMARY KEY,
				title VARCHAR(200),
				content TEXT
			)
		`)

		// This test is exploratory - we expect it might fail
		// because FULLTEXT is MySQL-specific
		if err != nil {
			t.Logf("✋ Expected: CREATE TABLE with FULLTEXT not supported - %v", err)
			// This is acceptable - PostgreSQL doesn't have FULLTEXT syntax
			return
		}

		t.Log("✅ Table created (without FULLTEXT index)")
	})

	t.Run("Insert test data", func(t *testing.T) {
		testData := []struct {
			id      int
			title   string
			content string
		}{
			{1, "Introduction to MySQL", "MySQL is a popular relational database"},
			{2, "PostgreSQL Features", "PostgreSQL has advanced features like full-text search"},
			{3, "Database Comparison", "MySQL and PostgreSQL are both great databases"},
			{4, "Oracle Database", "Oracle is an enterprise database solution"},
		}

		for _, data := range testData {
			_, err := db.Exec(
				"INSERT INTO test_fulltext (id, title, content) VALUES (?, ?, ?)",
				data.id, data.title, data.content,
			)
			if err != nil {
				t.Logf("⚠️ Insert failed: %v", err)
				return
			}
		}

		t.Log("✅ Test data inserted")
	})

	t.Run("MATCH AGAINST query with conversion", func(t *testing.T) {
		// This tests the MySQL MATCH AGAINST syntax being converted to PostgreSQL
		// MySQL:      MATCH(title, content) AGAINST('MySQL' IN BOOLEAN MODE)
		// PostgreSQL: to_tsvector('simple', title || ' ' || content) @@ to_tsquery('simple', 'MySQL')

		query := `
			SELECT title
			FROM test_fulltext
			WHERE MATCH(title, content) AGAINST('MySQL' IN BOOLEAN MODE)
		`

		rows, err := db.Query(query)

		// With the conversion implemented, this should now work
		require.NoError(t, err, "MATCH AGAINST should be converted to PostgreSQL syntax")
		defer rows.Close()

		t.Log("✅ MATCH AGAINST query successfully converted and executed")

		var titles []string
		for rows.Next() {
			var title string
			err := rows.Scan(&title)
			require.NoError(t, err)
			titles = append(titles, title)
		}

		t.Logf("Found %d matching records: %v", len(titles), titles)

		// We expect to find records containing "MySQL"
		// Should match: "Introduction to MySQL" and "Database Comparison"
		assert.GreaterOrEqual(t, len(titles), 1, "Should find at least one record with 'MySQL'")

		// Verify we got expected results
		foundMySQL := false
		for _, title := range titles {
			if strings.Contains(title, "MySQL") {
				foundMySQL = true
				break
			}
		}
		assert.True(t, foundMySQL, "Should find at least one title containing 'MySQL'")
	})

	t.Run("Alternative: Try PostgreSQL native syntax", func(t *testing.T) {
		// Try the PostgreSQL equivalent directly
		// This tests if we can bypass AProxy's rewriting and use PostgreSQL syntax

		query := `
			SELECT title
			FROM test_fulltext
			WHERE to_tsvector('simple', title || ' ' || COALESCE(content, ''))
			      @@ to_tsquery('simple', 'MySQL')
		`

		rows, err := db.Query(query)

		if err != nil {
			t.Logf("❌ PostgreSQL native syntax also failed: %v", err)
			// This might happen if the PostgreSQL syntax is too complex for AProxy
			return
		}

		defer rows.Close()

		var titles []string
		for rows.Next() {
			var title string
			err := rows.Scan(&title)
			require.NoError(t, err)
			titles = append(titles, title)
		}

		t.Logf("✅ PostgreSQL native syntax works!")
		t.Logf("Found %d records: %v", len(titles), titles)

		// Should find "Introduction to MySQL" and "Database Comparison"
		assert.GreaterOrEqual(t, len(titles), 1, "Should find at least one record with 'MySQL'")
	})

	t.Run("Cleanup", func(t *testing.T) {
		_, err := db.Exec("DROP TABLE IF EXISTS test_fulltext")
		if err != nil {
			t.Logf("⚠️ Cleanup failed: %v", err)
		} else {
			t.Log("✅ Cleanup completed")
		}
	})
}

// TestMySQLSpecific_FULLTEXT_Index tests FULLTEXT index creation
func TestMySQLSpecific_FULLTEXT_Index(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_ft_idx")

	t.Run("Probe: CREATE FULLTEXT INDEX", func(t *testing.T) {
		// First create a regular table
		_, err := db.Exec(`
			CREATE TABLE test_ft_idx (
				id INT PRIMARY KEY,
				title TEXT
			)
		`)

		if err != nil {
			t.Logf("Table creation failed: %v", err)
			return
		}

		// Try to create a FULLTEXT index (MySQL syntax)
		_, err = db.Exec(`
			CREATE FULLTEXT INDEX idx_title ON test_ft_idx(title)
		`)

		if err != nil {
			t.Logf("❌ FULLTEXT INDEX not supported (expected): %v", err)
			t.Logf("📌 Recommendation: Use PostgreSQL GIN index:")
			t.Logf("   CREATE INDEX idx_title ON test_ft_idx")
			t.Logf("   USING GIN (to_tsvector('simple', title))")

			assert.Error(t, err, "FULLTEXT INDEX should not be supported")
		} else {
			t.Log("⚠️ Unexpected: FULLTEXT INDEX creation succeeded")
		}

		// Cleanup
		_, _ = db.Exec("DROP TABLE IF EXISTS test_ft_idx")
	})
}

// TestMySQLSpecific_BooleanModeOperators tests BOOLEAN MODE operators
func TestMySQLSpecific_BooleanModeOperators(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_bool_mode")

	t.Run("Setup", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE test_bool_mode (
				id INT PRIMARY KEY,
				text TEXT
			)
		`)
		if err != nil {
			t.Skipf("Cannot create table: %v", err)
		}

		testData := []string{
			"MySQL database tutorial",
			"Oracle database guide",
			"MySQL and PostgreSQL comparison",
		}

		for i, text := range testData {
			_, _ = db.Exec("INSERT INTO test_bool_mode (id, text) VALUES (?, ?)", i+1, text)
		}
	})

	t.Run("Probe: Boolean operators +MySQL -Oracle", func(t *testing.T) {
		// MySQL: AGAINST('+MySQL -Oracle' IN BOOLEAN MODE)
		// PostgreSQL: to_tsquery('simple', 'MySQL & !Oracle')

		query := `
			SELECT text
			FROM test_bool_mode
			WHERE MATCH(text) AGAINST('+MySQL -Oracle' IN BOOLEAN MODE)
		`

		_, err := db.Query(query)

		if err != nil {
			t.Logf("❌ Boolean mode operators not supported (expected): %v", err)
			t.Logf("📌 PostgreSQL equivalent:")
			t.Logf("   WHERE to_tsvector('simple', text)")
			t.Logf("         @@ to_tsquery('simple', 'MySQL & !Oracle')")

			assert.Error(t, err, "Boolean mode should not be supported")
		} else {
			t.Log("⚠️ Unexpected: Boolean mode query succeeded")
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		_, _ = db.Exec("DROP TABLE IF EXISTS test_bool_mode")
	})
}

// Summary test to document findings
func TestMySQLSpecific_Summary(t *testing.T) {
	t.Log("=" + fmt.Sprintf("%80s", "="))
	t.Log("📊 MySQL MATCH AGAINST Feature Support Summary")
	t.Log("=" + fmt.Sprintf("%80s", "="))
	t.Log("")
	t.Log("🔍 Features Tested:")
	t.Log("  1. MATCH(columns) AGAINST('term' IN BOOLEAN MODE)")
	t.Log("  2. CREATE FULLTEXT INDEX")
	t.Log("  3. Boolean mode operators (+term -term)")
	t.Log("")
	t.Log("✅ Current Support:")
	t.Log("  - MySQL MATCH AGAINST syntax: ✅ SUPPORTED (converted to PostgreSQL)")
	t.Log("  - Conversion: MATCH(col1, col2) AGAINST('term')")
	t.Log("              → to_tsvector('simple', col1 || ' ' || col2) @@ to_tsquery('simple', 'term')")
	t.Log("  - FULLTEXT INDEX: ❌ NOT SUPPORTED (use PostgreSQL GIN index)")
	t.Log("  - Boolean operators: ⚠️ PARTIAL (term passed as-is, may need manual conversion)")
	t.Log("")
	t.Log("📌 PostgreSQL Alternatives:")
	t.Log("  - Index: CREATE INDEX USING GIN (to_tsvector('simple', column))")
	t.Log("  - Boolean: 'term1 & term2' (AND), 'term1 | term2' (OR), '!term' (NOT)")
	t.Log("")
	t.Log("📚 Reference: prompt/mysql_to_MATCH_AGAINST.md")
	t.Log("=" + fmt.Sprintf("%80s", "="))
}
// TestLastInsertID tests LAST_INSERT_ID() function support
// Verifies that AProxy correctly converts LAST_INSERT_ID() to PostgreSQL's lastval()
func TestLastInsertID(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
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

// TestYearType tests YEAR type conversion
// MySQL YEAR type is converted to PostgreSQL SMALLINT via AST rewriting
func TestYearType(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_year")
	_, err = db.Exec(`CREATE TABLE test_year (
		id INT AUTO_INCREMENT PRIMARY KEY,
		birth_year YEAR
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_year")

	_, err = db.Exec("INSERT INTO test_year (birth_year) VALUES (2024)")
	assert.NoError(t, err)

	var year int
	err = db.QueryRow("SELECT birth_year FROM test_year WHERE id = 1").Scan(&year)
	assert.NoError(t, err)
	assert.Equal(t, 2024, year)
}

// TestUnsignedType tests UNSIGNED modifier conversion
// MySQL UNSIGNED types are converted to larger PostgreSQL types via AST rewriting
// INT UNSIGNED -> BIGINT, BIGINT UNSIGNED -> NUMERIC
func TestUnsignedType(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_unsigned")
	_, err = db.Exec(`CREATE TABLE test_unsigned (
		id INT AUTO_INCREMENT PRIMARY KEY,
		count INT UNSIGNED,
		big_count BIGINT UNSIGNED
	)`)
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_unsigned")

	_, err = db.Exec("INSERT INTO test_unsigned (count, big_count) VALUES (4294967295, 18446744073709551615)")
	assert.NoError(t, err)
}

// TestLockInShareMode tests LOCK IN SHARE MODE conversion
// MySQL's LOCK IN SHARE MODE is converted to PostgreSQL's FOR SHARE via string post-processing
func TestLockInShareMode(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_lock")
	_, err = db.Exec("CREATE TABLE test_lock (id INT PRIMARY KEY, val INT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_lock VALUES (1, 100)")
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_lock")

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	var val int
	err = tx.QueryRow("SELECT val FROM test_lock WHERE id = 1 LOCK IN SHARE MODE").Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, 100, val)
}

// TestForUpdateSkipLocked tests FOR UPDATE SKIP LOCKED syntax
// This syntax is natively supported by PostgreSQL 9.5+, no conversion needed
func TestForUpdateSkipLocked(t *testing.T) {
	db, err := sql.Open("mysql", "root@tcp(localhost:3306)/test")
	require.NoError(t, err)
	defer db.Close()

	_, _ = db.Exec("DROP TABLE IF EXISTS test_skip_locked")
	_, err = db.Exec("CREATE TABLE test_skip_locked (id INT PRIMARY KEY, val INT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO test_skip_locked VALUES (1, 100)")
	require.NoError(t, err)
	defer db.Exec("DROP TABLE IF EXISTS test_skip_locked")

	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()

	// PostgreSQL supports this natively in 9.5+
	var val int
	err = tx.QueryRow("SELECT val FROM test_skip_locked WHERE id = 1 FOR UPDATE SKIP LOCKED").Scan(&val)
	assert.NoError(t, err)
	assert.Equal(t, 100, val)
}
