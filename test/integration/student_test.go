package integration

import (
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStudentTable comprehensive tests for student table
func TestStudentTable(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "students")

	t.Run("Create student table", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE students (
				id INT AUTO_INCREMENT PRIMARY KEY,
				student_no VARCHAR(20) UNIQUE NOT NULL,
				name VARCHAR(100) NOT NULL,
				age TINYINT UNSIGNED,
				gender ENUM('M', 'F'),
				grade TINYINT,
				class_name VARCHAR(50),
				email VARCHAR(100),
				phone VARCHAR(20),
				address TEXT,
				score DECIMAL(5,2),
				enrollment_date DATE,
				created_at DATETIME DEFAULT NOW(),
				updated_at DATETIME DEFAULT NOW(),
				INDEX idx_student_no (student_no),
				INDEX idx_grade_class (grade, class_name)
			) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
		`)
		assert.NoError(t, err)
	})

	t.Run("Insert 100 student records", func(t *testing.T) {
		for i := 1; i <= 100; i++ {
			studentNo := fmt.Sprintf("STU%05d", i)
			name := fmt.Sprintf("Student%d", i)
			age := 15 + (i % 8) // 15-22 years old
			gender := "M"
			if i%2 == 0 {
				gender = "F"
			}
			grade := 1 + (i % 6) // 1-6Grade
			className := fmt.Sprintf("%dGrade%dClass", grade, 1+(i%4))
			email := fmt.Sprintf("student%d@school.com", i)
			phone := fmt.Sprintf("138%08d", i)
			address := fmt.Sprintf("Beijing Haidian District Street%d", i%20)
			score := 60.0 + float64(i%40)
			enrollmentDate := time.Date(2020, time.September, 1, 0, 0, 0, 0, time.UTC).
				AddDate(0, 0, i%365)

			_, err := db.Exec(`
				INSERT INTO students (
					student_no, name, age, gender, grade, class_name,
					email, phone, address, score, enrollment_date
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`, studentNo, name, age, gender, grade, className, email, phone, address, score, enrollmentDate)

			require.NoError(t, err, "Failed to insert student %d", i)
		}

		// Verify number of inserted records
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM students").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 100, count, "Should insert 100 students")
	})

	t.Run("Query student data", func(t *testing.T) {
		// Query first student
		var id int
		var studentNo, name, email string
		var age int
		var score float64

		err := db.QueryRow(`
			SELECT id, student_no, name, age, email, score
			FROM students
			WHERE student_no = ?
		`, "STU00001").Scan(&id, &studentNo, &name, &age, &email, &score)

		assert.NoError(t, err)
		assert.Equal(t, "STU00001", studentNo)
		assert.Equal(t, "Student1", name)
		assert.Equal(t, "student1@school.com", email)
	})

	t.Run("Update student data", func(t *testing.T) {
		// Update first student score
		result, err := db.Exec(`
			UPDATE students
			SET score = ?, updated_at = NOW()
			WHERE student_no = ?
		`, 95.5, "STU00001")

		assert.NoError(t, err)
		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		// Verify update
		var score float64
		err = db.QueryRow("SELECT score FROM students WHERE student_no = ?", "STU00001").Scan(&score)
		assert.NoError(t, err)
		assert.Equal(t, 95.5, score)
	})

	t.Run("Delete student data", func(t *testing.T) {
		// Delete last student
		result, err := db.Exec("DELETE FROM students WHERE student_no = ?", "STU00100")
		assert.NoError(t, err)

		affected, err := result.RowsAffected()
		assert.NoError(t, err)
		assert.Equal(t, int64(1), affected)

		// Verify deletion
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM students").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 99, count)
	})

	t.Run("Aggregate query - group by grade", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT grade, COUNT(*) as student_count, AVG(score) as avg_score
			FROM students
			GROUP BY grade
			ORDER BY grade
		`)
		require.NoError(t, err)
		defer rows.Close()

		gradeCount := 0
		for rows.Next() {
			var grade, count int
			var avgScore float64
			err := rows.Scan(&grade, &count, &avgScore)
			assert.NoError(t, err)
			assert.Greater(t, count, 0)
			gradeCount++
		}
		assert.Greater(t, gradeCount, 0, "Should have statistics for multiple grades")
	})

	t.Run("Complex query - multiple conditions", func(t *testing.T) {
		rows, err := db.Query(`
			SELECT student_no, name, score
			FROM students
			WHERE grade >= ? AND score > ?
			ORDER BY score DESC
			LIMIT 10
		`, 3, 80.0)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var studentNo, name string
			var score float64
			err := rows.Scan(&studentNo, &name, &score)
			assert.NoError(t, err)
			assert.Greater(t, score, 80.0)
			count++
		}
		assert.Greater(t, count, 0, "Should query students matching conditions")
	})
}

// TestStudentTransactions tests transaction functionality for student table
func TestStudentTransactions(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "students")

	// Create student table
	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			score DECIMAL(5,2),
			credits INT DEFAULT 0
		)
	`)
	require.NoError(t, err)

	t.Run("Transaction commit - transfer credits", func(t *testing.T) {
		// Insert two students
		_, err := db.Exec("INSERT INTO students (student_no, name, score, credits) VALUES (?, ?, ?, ?)",
			"STU001", "Zhang San", 85.5, 100)
		require.NoError(t, err)
		_, err = db.Exec("INSERT INTO students (student_no, name, score, credits) VALUES (?, ?, ?, ?)",
			"STU002", "Li Si", 90.0, 50)
		require.NoError(t, err)

		// Begin transaction: Zhang San transfers 20 credits to Li Si
		tx, err := db.Begin()
		require.NoError(t, err)

		// Deduct Zhang San credits
		_, err = tx.Exec("UPDATE students SET credits = credits - ? WHERE student_no = ?", 20, "STU001")
		require.NoError(t, err)

		// Add Li Si credits
		_, err = tx.Exec("UPDATE students SET credits = credits + ? WHERE student_no = ?", 20, "STU002")
		require.NoError(t, err)

		// Commit transaction
		err = tx.Commit()
		assert.NoError(t, err)

		// Verify result
		var credits1, credits2 int
		err = db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU001").Scan(&credits1)
		assert.NoError(t, err)
		assert.Equal(t, 80, credits1)

		err = db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU002").Scan(&credits2)
		assert.NoError(t, err)
		assert.Equal(t, 70, credits2)
	})

	t.Run("Transaction rollback - invalid transfer", func(t *testing.T) {
		// Record current credits
		var creditsBefore1, creditsBefore2 int
		err := db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU001").Scan(&creditsBefore1)
		require.NoError(t, err)
		err = db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU002").Scan(&creditsBefore2)
		require.NoError(t, err)

		// Begin transaction: try to transfer more than balance
		tx, err := db.Begin()
		require.NoError(t, err)

		// Deduct credits (would cause negative)
		_, err = tx.Exec("UPDATE students SET credits = credits - ? WHERE student_no = ?", 1000, "STU001")
		require.NoError(t, err)

		// Error detected, rollback transaction
		err = tx.Rollback()
		assert.NoError(t, err)

		// Verify data unchanged
		var creditsAfter1, creditsAfter2 int
		err = db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU001").Scan(&creditsAfter1)
		assert.NoError(t, err)
		assert.Equal(t, creditsBefore1, creditsAfter1, "Credits should remain unchanged after rollback")

		err = db.QueryRow("SELECT credits FROM students WHERE student_no = ?", "STU002").Scan(&creditsAfter2)
		assert.NoError(t, err)
		assert.Equal(t, creditsBefore2, creditsAfter2, "Credits should remain unchanged after rollback")
	})

	t.Run("Explicit transaction control - BEGIN/COMMIT", func(t *testing.T) {
		// Use explicit BEGIN/COMMIT statements
		_, err := db.Exec("BEGIN")
		require.NoError(t, err)

		_, err = db.Exec("INSERT INTO students (student_no, name, score, credits) VALUES (?, ?, ?, ?)",
			"STU003", "Wang Wu", 88.0, 60)
		require.NoError(t, err)

		_, err = db.Exec("COMMIT")
		require.NoError(t, err)

		// Verify insertion success
		var name string
		err = db.QueryRow("SELECT name FROM students WHERE student_no = ?", "STU003").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "Wang Wu", name)
	})

	t.Run("Explicit transaction control - BEGIN/ROLLBACK", func(t *testing.T) {
		_, err := db.Exec("BEGIN")
		require.NoError(t, err)

		_, err = db.Exec("INSERT INTO students (student_no, name, score, credits) VALUES (?, ?, ?, ?)",
			"STU004", "Zhao Liu", 92.0, 80)
		require.NoError(t, err)

		_, err = db.Exec("ROLLBACK")
		require.NoError(t, err)

		// Verify insertion was rolled back
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM students WHERE student_no = ?", "STU004").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count, "Should not have this record after rollback")
	})

	t.Run("START TRANSACTION syntax", func(t *testing.T) {
		_, err := db.Exec("START TRANSACTION")
		require.NoError(t, err)

		_, err = db.Exec("INSERT INTO students (student_no, name, score, credits) VALUES (?, ?, ?, ?)",
			"STU005", "Sun Qi", 87.5, 70)
		require.NoError(t, err)

		_, err = db.Exec("COMMIT")
		require.NoError(t, err)

		// Verify insertion success
		var name string
		err = db.QueryRow("SELECT name FROM students WHERE student_no = ?", "STU005").Scan(&name)
		assert.NoError(t, err)
		assert.Equal(t, "Sun Qi", name)
	})
}

// TestStudentAutocommit tests autocommit mode
func TestStudentAutocommit(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "students")

	// Create student table
	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL
		)
	`)
	require.NoError(t, err)

	t.Run("Disable autocommit and manual commit", func(t *testing.T) {
		// Disable autocommit
		_, err := db.Exec("SET AUTOCOMMIT = 0")
		require.NoError(t, err)

		// Insert data (will not auto commit)
		_, err = db.Exec("INSERT INTO students (student_no, name) VALUES (?, ?)", "STU001", "Test1")
		require.NoError(t, err)

		// Commit
		_, err = db.Exec("COMMIT")
		require.NoError(t, err)

		// Verify data committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM students WHERE student_no = ?", "STU001").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)

		// Restore autocommit
		_, err = db.Exec("SET AUTOCOMMIT = 1")
		require.NoError(t, err)
	})

	t.Run("Enable autocommit", func(t *testing.T) {
		// Ensure autocommit is enabled
		_, err := db.Exec("SET AUTOCOMMIT = ON")
		require.NoError(t, err)

		// Insert data (auto commit)
		_, err = db.Exec("INSERT INTO students (student_no, name) VALUES (?, ?)", "STU002", "Test2")
		require.NoError(t, err)

		// Verify data auto committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM students WHERE student_no = ?", "STU002").Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}

// TestStudentSQLRewrite tests SQL rewrite functionality
func TestStudentSQLRewrite(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "students")

	t.Run("Data type conversion", func(t *testing.T) {
		_, err := db.Exec(`
			CREATE TABLE students (
				id INT AUTO_INCREMENT PRIMARY KEY,
				age TINYINT,
				score FLOAT,
				gpa DOUBLE,
				enrollment_date DATETIME,
				bio LONGTEXT
			)
		`)
		assert.NoError(t, err, "Should successfully create table (data types auto converted to PG types)")
	})

	t.Run("Function conversion", func(t *testing.T) {
		// Insert test data
		_, err := db.Exec(`
			INSERT INTO students (age, score, gpa, enrollment_date, bio)
			VALUES (?, ?, ?, NOW(), ?)
		`, 18, 85.5, 3.5, "StudentBio")
		assert.NoError(t, err)

		// Test function conversion
		var nowStr string
		err = db.QueryRow("SELECT NOW()").Scan(&nowStr)
		assert.NoError(t, err)
		// Parse the string to time.Time for comparison (PostgreSQL returns ISO 8601 format)
		now, err := time.Parse(time.RFC3339, nowStr)
		if err != nil {
			// Try parsing without timezone - assume local time
			now, err = time.ParseInLocation("2006-01-02 15:04:05", nowStr, time.Local)
			if err != nil {
				// Try with UTC as fallback
				now, err = time.ParseInLocation("2006-01-02 15:04:05", nowStr, time.UTC)
			}
		}
		assert.NoError(t, err)
		// Compare times using Unix timestamps (timezone-independent)
		// Convert both to UTC for comparison
		// Use large delta to handle timezone differences between test and PostgreSQL server
		nowUTC := time.Now().UTC()
		nowParsedUTC := now.UTC()
		delta := float64(nowUTC.Unix()) - float64(nowParsedUTC.Unix())
		// Allow up to 9 hours difference to handle timezone issues (CST+8, etc.)
		assert.True(t, delta >= -32400 && delta <= 32400, "NOW() should return current time (delta: %.0f seconds)", delta)

		var dateStr string
		err = db.QueryRow("SELECT CURDATE()").Scan(&dateStr)
		assert.NoError(t, err)

		var length int
		err = db.QueryRow("SELECT LENGTH(bio) FROM students WHERE age = ?", 18).Scan(&length)
		assert.NoError(t, err)
		assert.Greater(t, length, 0)
	})

	t.Run("LIMIT syntax conversion", func(t *testing.T) {
		// Insert multiple records
		for i := 1; i <= 20; i++ {
			_, err := db.Exec("INSERT INTO students (age, score, gpa, enrollment_date, bio) VALUES (?, ?, ?, NOW(), ?)",
				18+i%5, 70.0+float64(i), 3.0+float64(i)/10, fmt.Sprintf("Student%d", i))
			require.NoError(t, err)
		}

		// Test MySQL LIMIT offset, count syntax
		rows, err := db.Query("SELECT age FROM students ORDER BY age LIMIT 5, 10")
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var age int
			err := rows.Scan(&age)
			assert.NoError(t, err)
			count++
		}
		assert.Equal(t, 10, count, "Should return 10 records (offset 5)")
	})

	t.Run("Backtick conversion", func(t *testing.T) {
		// Query using backticks
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM `students` WHERE `age` > ?", 18).Scan(&count)
		assert.NoError(t, err)
		assert.Greater(t, count, 0)
	})
}

// TestStudentConcurrentTransactions tests concurrent transactions
func TestStudentConcurrentTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip concurrent test")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "students")

	// Create student table
	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			balance INT DEFAULT 0
		)
	`)
	require.NoError(t, err)

	// Insert initial data
	_, err = db.Exec("INSERT INTO students (student_no, name, balance) VALUES (?, ?, ?)",
		"BANK001", "Account1", 1000)
	require.NoError(t, err)

	t.Run("Concurrent transfers", func(t *testing.T) {
		done := make(chan bool, 10)

		// 10 concurrent transactions, each transfers 10
		for i := 0; i < 10; i++ {
			go func(idx int) {
				tx, err := db.Begin()
				if err != nil {
					t.Errorf("Failed to begin transaction: %v", err)
					done <- false
					return
				}

				// Deduct balance
				_, err = tx.Exec("UPDATE students SET balance = balance - ? WHERE student_no = ?", 10, "BANK001")
				if err != nil {
					tx.Rollback()
					t.Errorf("Failed to update: %v", err)
					done <- false
					return
				}

				// Commit
				err = tx.Commit()
				if err != nil {
					t.Errorf("Failed to commit: %v", err)
					done <- false
					return
				}

				done <- true
			}(i)
		}

		// Wait for all transactions to complete
		successCount := 0
		for i := 0; i < 10; i++ {
			if <-done {
				successCount++
			}
		}

		t.Logf("Number of successful transactions: %d", successCount)

		// Verify final balance
		var balance int
		err = db.QueryRow("SELECT balance FROM students WHERE student_no = ?", "BANK001").Scan(&balance)
		assert.NoError(t, err)
		expectedBalance := 1000 - (successCount * 10)
		assert.Equal(t, expectedBalance, balance, "Balance should be correctly reduced")
	})
}

// TestStudentComplexScenarios tests complex scenarios
func TestStudentComplexScenarios(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case tables exist from previous test
	cleanupPostgreSQL(t, "enrollments")
	cleanupPostgreSQL(t, "courses")
	cleanupPostgreSQL(t, "students")
	defer cleanupPostgreSQL(t, "enrollments")
	defer cleanupPostgreSQL(t, "courses")
	defer cleanupPostgreSQL(t, "students")

	// Create student table
	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			grade TINYINT
		)
	`)
	require.NoError(t, err)

	// Create courses table
	_, err = db.Exec(`
		CREATE TABLE courses (
			id INT AUTO_INCREMENT PRIMARY KEY,
			course_code VARCHAR(20) UNIQUE NOT NULL,
			course_name VARCHAR(100) NOT NULL,
			credits TINYINT
		)
	`)
	require.NoError(t, err)

	// Create enrollments table
	_, err = db.Exec(`
		CREATE TABLE enrollments (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_id INT,
			course_id INT,
			score DECIMAL(5,2),
			enrolled_at DATETIME DEFAULT NOW()
		)
	`)
	require.NoError(t, err)

	t.Run("Complex transaction - course enrollment", func(t *testing.T) {
		// Insert student
		result, err := db.Exec("INSERT INTO students (student_no, name, grade) VALUES (?, ?, ?)",
			"STU001", "Zhang San", 1)
		require.NoError(t, err)
		studentID, _ := result.LastInsertId()

		// Insert course
		result, err = db.Exec("INSERT INTO courses (course_code, course_name, credits) VALUES (?, ?, ?)",
			"CS101", "Computer Fundamentals", 3)
		require.NoError(t, err)
		courseID, _ := result.LastInsertId()

		// Begin transaction for enrollment
		tx, err := db.Begin()
		require.NoError(t, err)

		// Check if already enrolled
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM enrollments WHERE student_id = ? AND course_id = ?",
			studentID, courseID).Scan(&count)
		require.NoError(t, err)

		if count == 0 {
			// Enroll
			_, err = tx.Exec("INSERT INTO enrollments (student_id, course_id) VALUES (?, ?)",
				studentID, courseID)
			require.NoError(t, err)
		}

		err = tx.Commit()
		assert.NoError(t, err)

		// Verify enrollment success
		err = db.QueryRow("SELECT COUNT(*) FROM enrollments WHERE student_id = ? AND course_id = ?",
			studentID, courseID).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("JOIN query - student enrollment info", func(t *testing.T) {
		// The previous test (Complex transaction - course enrollment) should have created enrollment data
		// Just run a simple JOIN query to verify it works
		rows, err := db.Query(`
			SELECT s.student_no, s.name, c.course_code, c.course_name
			FROM students s
			JOIN enrollments e ON s.id = e.student_id
			JOIN courses c ON e.course_id = c.id
			WHERE s.student_no = 'STU001'
			ORDER BY s.student_no
		`)
		require.NoError(t, err)
		defer rows.Close()

		// We expect at least 1 record from the previous test
		found := false
		for rows.Next() {
			var studentNo, name, courseCode, courseName string
			err := rows.Scan(&studentNo, &name, &courseCode, &courseName)
			assert.NoError(t, err)
			assert.Equal(t, "STU001", studentNo)
			found = true
		}
		assert.True(t, found, "Should find enrollment for STU001 from previous test")
	})
}

// BenchmarkStudentInsert benchmark for batch insert performance
func BenchmarkStudentInsert(b *testing.B) {
	db, cleanup := setupTestDB(b)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(b, "students")
	defer cleanupPostgreSQL(b, "students")

	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			name VARCHAR(100) NOT NULL,
			score DECIMAL(5,2)
		)
	`)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		studentNo := fmt.Sprintf("STU%08d", i)
		name := fmt.Sprintf("Student%d", i)
		score := 60.0 + float64(i%40)

		db.Exec("INSERT INTO students (student_no, name, score) VALUES (?, ?, ?)",
			studentNo, name, score)
	}
}

// BenchmarkStudentTransaction benchmark for transaction performance
func BenchmarkStudentTransaction(b *testing.B) {
	db, cleanup := setupTestDB(b)
	defer cleanup()

	// Clean up first in case table exists from previous test
	cleanupPostgreSQL(b, "students")
	defer cleanupPostgreSQL(b, "students")

	_, err := db.Exec(`
		CREATE TABLE students (
			id INT AUTO_INCREMENT PRIMARY KEY,
			student_no VARCHAR(20) UNIQUE NOT NULL,
			balance INT DEFAULT 1000
		)
	`)
	if err != nil {
		b.Fatal(err)
	}

	// Insert test data
	db.Exec("INSERT INTO students (student_no) VALUES (?)", "STU001")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx, _ := db.Begin()
		tx.Exec("UPDATE students SET balance = balance - 1 WHERE student_no = ?", "STU001")
		tx.Commit()
	}
}
