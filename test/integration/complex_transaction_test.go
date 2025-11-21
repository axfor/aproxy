package integration

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLargeTransactionBatchInsert tests transaction with large batch insert
func TestLargeTransactionBatchInsert(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "large_users")

	// Create table
	_, err := db.Exec(`
		CREATE TABLE large_users (
			id INT AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(100) NOT NULL,
			email VARCHAR(100) NOT NULL,
			balance DECIMAL(10, 2) DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	t.Run("Insert 1000 records in single transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		start := time.Now()
		const batchSize = 1000

		for i := 0; i < batchSize; i++ {
			_, err = tx.Exec(
				"INSERT INTO large_users (username, email, balance) VALUES (?, ?, ?)",
				fmt.Sprintf("user_%d", i),
				fmt.Sprintf("user_%d@example.com", i),
				float64(i)*10.5,
			)
			if err != nil {
				tx.Rollback()
				require.NoError(t, err, "Failed at iteration %d", i)
			}
		}

		err = tx.Commit()
		require.NoError(t, err)

		elapsed := time.Since(start)
		t.Logf("Inserted %d records in %v", batchSize, elapsed)

		// Verify count
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM large_users").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, batchSize, count)
	})

	t.Run("Batch update in transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		start := time.Now()
		const updateSize = 500

		for i := 0; i < updateSize; i++ {
			_, err = tx.Exec(
				"UPDATE large_users SET balance = balance + ? WHERE username = ?",
				100.0,
				fmt.Sprintf("user_%d", i),
			)
			if err != nil {
				tx.Rollback()
				require.NoError(t, err)
			}
		}

		err = tx.Commit()
		require.NoError(t, err)

		elapsed := time.Since(start)
		t.Logf("Updated %d records in %v", updateSize, elapsed)

		// Verify first user balance
		var balance float64
		err = db.QueryRow("SELECT balance FROM large_users WHERE username = ?", "user_0").Scan(&balance)
		require.NoError(t, err)
		assert.Equal(t, 100.0, balance)
	})
}

// TestComplexEcommerceTransaction tests complex e-commerce order scenario
func TestComplexEcommerceTransaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer func() {
		cleanupPostgreSQL(t, "order_items")
		cleanupPostgreSQL(t, "orders")
		cleanupPostgreSQL(t, "inventory")
		cleanupPostgreSQL(t, "products")
		cleanupPostgreSQL(t, "customers")
	}()

	// Create schema
	_, err := db.Exec(`
		CREATE TABLE customers (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			balance DECIMAL(10, 2) DEFAULT 0
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE products (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			price DECIMAL(10, 2) NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE inventory (
			product_id INT PRIMARY KEY,
			quantity INT NOT NULL DEFAULT 0
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE orders (
			id INT AUTO_INCREMENT PRIMARY KEY,
			customer_id INT NOT NULL,
			total_amount DECIMAL(10, 2) NOT NULL,
			status VARCHAR(20) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE order_items (
			id INT AUTO_INCREMENT PRIMARY KEY,
			order_id INT NOT NULL,
			product_id INT NOT NULL,
			quantity INT NOT NULL,
			price DECIMAL(10, 2) NOT NULL
		)
	`)
	require.NoError(t, err)

	// Initialize test data
	_, err = db.Exec("INSERT INTO customers (name, balance) VALUES (?, ?)", "Alice", 10000.00)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO products (name, price) VALUES (?, ?)", "Laptop", 1200.50)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO products (name, price) VALUES (?, ?)", "Mouse", 25.99)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO products (name, price) VALUES (?, ?)", "Keyboard", 89.99)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO inventory (product_id, quantity) VALUES (?, ?)", 1, 100)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO inventory (product_id, quantity) VALUES (?, ?)", 2, 500)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO inventory (product_id, quantity) VALUES (?, ?)", 3, 200)
	require.NoError(t, err)

	t.Run("Complete order transaction", func(t *testing.T) {
		// Simulate: Customer places an order for multiple items
		// 1. Check inventory
		// 2. Deduct customer balance
		// 3. Reduce inventory
		// 4. Create order
		// 5. Create order items

		tx, err := db.Begin()
		require.NoError(t, err)

		// Get customer
		var customerID int
		var customerBalance float64
		err = tx.QueryRow("SELECT id, balance FROM customers WHERE name = ?", "Alice").Scan(&customerID, &customerBalance)
		require.NoError(t, err)

		// Items to order: 2 Laptops, 5 Mice, 3 Keyboards
		orderItems := []struct {
			productID int
			quantity  int
		}{
			{1, 2}, // 2 Laptops
			{2, 5}, // 5 Mice
			{3, 3}, // 3 Keyboards
		}

		totalAmount := 0.0

		// Check inventory and calculate total
		for _, item := range orderItems {
			var quantity int
			var price float64
			err = tx.QueryRow("SELECT quantity FROM inventory WHERE product_id = ?", item.productID).Scan(&quantity)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, quantity, item.quantity, "Insufficient inventory")

			err = tx.QueryRow("SELECT price FROM products WHERE id = ?", item.productID).Scan(&price)
			require.NoError(t, err)

			totalAmount += price * float64(item.quantity)
		}

		// Check customer balance
		if customerBalance < totalAmount {
			tx.Rollback()
			t.Fatalf("Insufficient balance: has %.2f, needs %.2f", customerBalance, totalAmount)
		}

		// Deduct customer balance
		_, err = tx.Exec("UPDATE customers SET balance = balance - ? WHERE id = ?", totalAmount, customerID)
		require.NoError(t, err)

		// Create order
		result, err := tx.Exec(
			"INSERT INTO orders (customer_id, total_amount, status) VALUES (?, ?, ?)",
			customerID, totalAmount, "confirmed",
		)
		require.NoError(t, err)

		orderID, err := result.LastInsertId()
		require.NoError(t, err)

		// Create order items and reduce inventory
		for _, item := range orderItems {
			var price float64
			err = tx.QueryRow("SELECT price FROM products WHERE id = ?", item.productID).Scan(&price)
			require.NoError(t, err)

			_, err = tx.Exec(
				"INSERT INTO order_items (order_id, product_id, quantity, price) VALUES (?, ?, ?, ?)",
				orderID, item.productID, item.quantity, price,
			)
			require.NoError(t, err)

			_, err = tx.Exec("UPDATE inventory SET quantity = quantity - ? WHERE product_id = ?", item.quantity, item.productID)
			require.NoError(t, err)
		}

		// Commit transaction
		err = tx.Commit()
		require.NoError(t, err)

		t.Logf("Order created successfully: ID=%d, Amount=%.2f", orderID, totalAmount)

		// Verify results
		var newBalance float64
		err = db.QueryRow("SELECT balance FROM customers WHERE id = ?", customerID).Scan(&newBalance)
		require.NoError(t, err)
		expectedBalance := customerBalance - totalAmount
		assert.Equal(t, expectedBalance, newBalance)

		// Verify inventory
		var laptopQty int
		err = db.QueryRow("SELECT quantity FROM inventory WHERE product_id = ?", 1).Scan(&laptopQty)
		require.NoError(t, err)
		assert.Equal(t, 98, laptopQty) // 100 - 2

		// Verify order items count
		var itemCount int
		err = db.QueryRow("SELECT COUNT(*) FROM order_items WHERE order_id = ?", orderID).Scan(&itemCount)
		require.NoError(t, err)
		assert.Equal(t, 3, itemCount)
	})
}

// TestTransactionIsolation tests transaction isolation levels
func TestTransactionIsolation(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "isolation_test")

	// Create table
	_, err := db.Exec(`
		CREATE TABLE isolation_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			value INT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO isolation_test (value) VALUES (?)", 100)
	require.NoError(t, err)

	t.Run("Dirty read prevention", func(t *testing.T) {
		tx1, err := db.Begin()
		require.NoError(t, err)
		defer tx1.Rollback()

		// TX1: Update but don't commit
		_, err = tx1.Exec("UPDATE isolation_test SET value = ? WHERE id = ?", 200, 1)
		require.NoError(t, err)

		// TX2: Should see old value (100), not uncommitted value (200)
		var value int
		err = db.QueryRow("SELECT value FROM isolation_test WHERE id = ?", 1).Scan(&value)
		require.NoError(t, err)
		assert.Equal(t, 100, value, "Should not see uncommitted changes")
	})

	t.Run("Read committed", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("UPDATE isolation_test SET value = ? WHERE id = ?", 300, 1)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Now should see committed value
		var value int
		err = db.QueryRow("SELECT value FROM isolation_test WHERE id = ?", 1).Scan(&value)
		require.NoError(t, err)
		assert.Equal(t, 300, value, "Should see committed changes")
	})
}

// TestDeadlockHandling tests deadlock detection and handling
func TestDeadlockHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip deadlock test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer func() {
		cleanupPostgreSQL(t, "account_a")
		cleanupPostgreSQL(t, "account_b")
	}()

	// Create two accounts
	_, err := db.Exec(`
		CREATE TABLE account_a (
			id INT PRIMARY KEY,
			balance INT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE account_b (
			id INT PRIMARY KEY,
			balance INT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO account_a (id, balance) VALUES (?, ?)", 1, 1000)
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO account_b (id, balance) VALUES (?, ?)", 1, 2000)
	require.NoError(t, err)

	t.Run("Deadlock scenario", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make(chan error, 2)

		// Transaction 1: A -> B
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				errors <- err
				return
			}
			defer tx.Rollback()

			_, err = tx.Exec("UPDATE account_a SET balance = balance - 100 WHERE id = 1")
			if err != nil {
				errors <- err
				return
			}

			time.Sleep(100 * time.Millisecond) // Increase chance of deadlock

			_, err = tx.Exec("UPDATE account_b SET balance = balance + 100 WHERE id = 1")
			if err != nil {
				errors <- err
				return
			}

			err = tx.Commit()
			errors <- err
		}()

		// Transaction 2: B -> A
		wg.Add(1)
		go func() {
			defer wg.Done()
			tx, err := db.Begin()
			if err != nil {
				errors <- err
				return
			}
			defer tx.Rollback()

			_, err = tx.Exec("UPDATE account_b SET balance = balance - 50 WHERE id = 1")
			if err != nil {
				errors <- err
				return
			}

			time.Sleep(100 * time.Millisecond) // Increase chance of deadlock

			_, err = tx.Exec("UPDATE account_a SET balance = balance + 50 WHERE id = 1")
			if err != nil {
				errors <- err
				return
			}

			err = tx.Commit()
			errors <- err
		}()

		wg.Wait()
		close(errors)

		// At least one transaction should complete
		successCount := 0
		deadlockCount := 0
		for err := range errors {
			if err == nil {
				successCount++
			} else {
				t.Logf("Transaction error: %v", err)
				deadlockCount++
			}
		}

		t.Logf("Success: %d, Errors: %d", successCount, deadlockCount)
		assert.GreaterOrEqual(t, successCount, 1, "At least one transaction should succeed")
	})
}

// TestLongRunningTransaction tests long-running transactions
func TestLongRunningTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("Skip long transaction test in short mode")
	}

	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "long_running_test")

	_, err := db.Exec(`
		CREATE TABLE long_running_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			data VARCHAR(100),
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	require.NoError(t, err)

	t.Run("Transaction with multiple operations over time", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		start := time.Now()

		// Insert records
		for i := 0; i < 10; i++ {
			_, err = tx.Exec("INSERT INTO long_running_test (data) VALUES (?)", fmt.Sprintf("data_%d", i))
			require.NoError(t, err)

			// Simulate processing time
			time.Sleep(50 * time.Millisecond)
		}

		// Update records
		_, err = tx.Exec("UPDATE long_running_test SET data = CONCAT(data, '_updated')")
		require.NoError(t, err)

		// Query within transaction
		var count int
		err = tx.QueryRow("SELECT COUNT(*) FROM long_running_test").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 10, count)

		err = tx.Commit()
		require.NoError(t, err)

		elapsed := time.Since(start)
		t.Logf("Long transaction completed in %v", elapsed)

		// Verify data
		var data string
		err = db.QueryRow("SELECT data FROM long_running_test WHERE id = 1").Scan(&data)
		require.NoError(t, err)
		assert.Equal(t, "data_0_updated", data)
	})
}

// TestBatchOperationsInTransaction tests various batch operations
func TestBatchOperationsInTransaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "batch_test")

	_, err := db.Exec(`
		CREATE TABLE batch_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			category VARCHAR(50),
			value INT NOT NULL,
			processed INT DEFAULT 0
		)
	`)
	require.NoError(t, err)

	t.Run("Batch insert then batch update", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		// Batch insert
		categories := []string{"A", "B", "C"}
		for _, cat := range categories {
			for i := 0; i < 100; i++ {
				_, err = tx.Exec(
					"INSERT INTO batch_test (category, value) VALUES (?, ?)",
					cat, i,
				)
				require.NoError(t, err)
			}
		}

		// Batch update by category
		_, err = tx.Exec("UPDATE batch_test SET processed = 1 WHERE category = ?", "A")
		require.NoError(t, err)

		// Conditional batch update
		_, err = tx.Exec("UPDATE batch_test SET value = value * 2 WHERE category = ? AND value < ?", "B", 50)
		require.NoError(t, err)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify processed count
		var processedCount int
		err = db.QueryRow("SELECT COUNT(*) FROM batch_test WHERE processed = 1").Scan(&processedCount)
		require.NoError(t, err)
		assert.Equal(t, 100, processedCount, "All category A records should be processed")

		// Verify value doubling
		var avgValue float64
		err = db.QueryRow("SELECT AVG(value) FROM batch_test WHERE category = ? AND id <= (SELECT MIN(id) + 49 FROM batch_test WHERE category = ?)", "B", "B").Scan(&avgValue)
		require.NoError(t, err)
		t.Logf("Average value for first 50 category B records: %.2f", avgValue)
	})

	t.Run("Batch delete in transaction", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		// Delete by condition
		result, err := tx.Exec("DELETE FROM batch_test WHERE category = ? AND value < ?", "C", 30)
		require.NoError(t, err)

		affected, err := result.RowsAffected()
		require.NoError(t, err)
		t.Logf("Deleted %d rows", affected)

		err = tx.Commit()
		require.NoError(t, err)

		// Verify deletion
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM batch_test WHERE category = ?", "C").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 70, count, "Should have 70 records left (100 - 30)")
	})
}

// TestTransactionRollbackScenarios tests various rollback scenarios
func TestTransactionRollbackScenarios(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	defer cleanupPostgreSQL(t, "rollback_test")

	_, err := db.Exec(`
		CREATE TABLE rollback_test (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) UNIQUE NOT NULL,
			status VARCHAR(20)
		)
	`)
	require.NoError(t, err)

	t.Run("Rollback on constraint violation", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		_, err = tx.Exec("INSERT INTO rollback_test (name, status) VALUES (?, ?)", "item1", "active")
		require.NoError(t, err)

		// Try to insert duplicate
		_, err = tx.Exec("INSERT INTO rollback_test (name, status) VALUES (?, ?)", "item1", "pending")
		if err != nil {
			t.Logf("Expected constraint violation: %v", err)
			tx.Rollback()
		}

		// Verify nothing was committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM rollback_test").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Transaction should be rolled back")
	})

	t.Run("Partial rollback simulation", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		// Insert first item
		_, err = tx.Exec("INSERT INTO rollback_test (name, status) VALUES (?, ?)", "item2", "active")
		require.NoError(t, err)

		// Insert second item
		_, err = tx.Exec("INSERT INTO rollback_test (name, status) VALUES (?, ?)", "item3", "pending")
		require.NoError(t, err)

		// Simulate business logic error
		shouldFail := true
		if shouldFail {
			tx.Rollback()
			t.Log("Transaction rolled back due to business logic")
		} else {
			tx.Commit()
		}

		// Verify nothing was committed
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM rollback_test").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count, "Both inserts should be rolled back")
	})
}

// TestMultiTableTransaction tests transactions spanning multiple tables
func TestMultiTableTransaction(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Clean up first in case tables exist from previous test
	cleanupPostgreSQL(t, "book_authors")
	cleanupPostgreSQL(t, "books")
	cleanupPostgreSQL(t, "authors")
	defer func() {
		cleanupPostgreSQL(t, "book_authors")
		cleanupPostgreSQL(t, "books")
		cleanupPostgreSQL(t, "authors")
	}()

	// Create schema
	_, err := db.Exec(`
		CREATE TABLE authors (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			book_count INT DEFAULT 0
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE books (
			id INT AUTO_INCREMENT PRIMARY KEY,
			title VARCHAR(200) NOT NULL,
			published_year INT
		)
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE book_authors (
			book_id INT NOT NULL,
			author_id INT NOT NULL,
			PRIMARY KEY (book_id, author_id)
		)
	`)
	require.NoError(t, err)

	t.Run("Insert book with multiple authors", func(t *testing.T) {
		tx, err := db.Begin()
		require.NoError(t, err)

		// Insert book
		result, err := tx.Exec(
			"INSERT INTO books (title, published_year) VALUES (?, ?)",
			"Database Systems", 2024,
		)
		require.NoError(t, err)

		bookID, err := result.LastInsertId()
		require.NoError(t, err)

		// Insert authors
		authors := []string{"Alice Smith", "Bob Johnson", "Carol Williams"}
		authorIDs := make([]int64, 0, len(authors))

		for _, name := range authors {
			result, err = tx.Exec("INSERT INTO authors (name, book_count) VALUES (?, ?)", name, 1)
			require.NoError(t, err)

			authorID, err := result.LastInsertId()
			require.NoError(t, err)
			authorIDs = append(authorIDs, authorID)

			// Link book and author
			_, err = tx.Exec(
				"INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)",
				bookID, authorID,
			)
			require.NoError(t, err)
		}

		err = tx.Commit()
		require.NoError(t, err)

		// Verify relationships
		var authorCount int
		err = db.QueryRow(`
			SELECT COUNT(*)
			FROM book_authors
			WHERE book_id = ?
		`, bookID).Scan(&authorCount)
		require.NoError(t, err)
		assert.Equal(t, 3, authorCount, "Book should have 3 authors")

		// Verify author count
		var totalAuthors int
		err = db.QueryRow("SELECT COUNT(*) FROM authors").Scan(&totalAuthors)
		require.NoError(t, err)
		assert.Equal(t, 3, totalAuthors)
	})
}
