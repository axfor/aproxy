package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	fmt.Println("=== Verify Text Format Configuration ===\n")

	// Test 1: Use Simple Query Protocol
	testSimpleProtocol()

	fmt.Println("\n==================================================\n")

	// Test 2: Use connection pool + Simple Protocol configuration
	testPoolWithSimpleProtocol()
}

func testSimpleProtocol() {
	fmt.Println("Test 1: Direct Simple Query Protocol")

	connString := "postgres://aproxy_user:aproxy_pass@localhost:5432/aproxy_test?sslmode=disable"

	connConfig, err := pgx.ParseConfig(connString)
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// Force use of Simple Query Protocol
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	fmt.Printf("DefaultQueryExecMode set to: %v\n", connConfig.DefaultQueryExecMode)

	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// Create test table
	_, err = conn.Exec(ctx, "DROP TABLE IF EXISTS test_format")
	if err != nil {
		log.Printf("Drop table warning: %v", err)
	}

	_, err = conn.Exec(ctx, `
		CREATE TABLE test_format (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			price DECIMAL(10,2)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	_, err = conn.Exec(ctx, "INSERT INTO test_format (name, price) VALUES ($1, $2)", "Product 1", 99.99)
	if err != nil {
		log.Fatalf("Failed to insert: %v", err)
	}

	// Query and check format
	rows, err := conn.Query(ctx, "SELECT * FROM test_format")
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	fmt.Println("\nField descriptions:")
	for i, fd := range fieldDescs {
		fmt.Printf("  [%d] Name: %s, DataTypeOID: %d, Format: %d",
			i, fd.Name, fd.DataTypeOID, fd.Format)
		if fd.Format == 0 {
			fmt.Print(" (Text Format ✓)")
		} else {
			fmt.Print(" (Binary Format ✗)")
		}
		fmt.Println()
	}

	// Read data
	if rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Fatalf("Failed to get values: %v", err)
		}
		fmt.Println("\nData values:")
		for i, v := range values {
			fmt.Printf("  [%d] Type: %T, Value: %v\n", i, v, v)
		}
	}
}

func testPoolWithSimpleProtocol() {
	fmt.Println("Test 2: Connection pool + Simple Query Protocol")

	connString := "postgres://aproxy_user:aproxy_pass@localhost:5432/aproxy_test?sslmode=disable"

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		log.Fatalf("Failed to parse pool config: %v", err)
	}

	// Force use of Simple Query Protocol
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	fmt.Printf("Pool DefaultQueryExecMode set to: %v\n", poolConfig.ConnConfig.DefaultQueryExecMode)

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Failed to create pool: %v", err)
	}
	defer pool.Close()

	// Acquire connection
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatalf("Failed to acquire connection: %v", err)
	}
	defer conn.Release()

	// Query and check format
	rows, err := conn.Query(ctx, "SELECT * FROM test_format")
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	fmt.Println("\nField descriptions:")
	for i, fd := range fieldDescs {
		fmt.Printf("  [%d] Name: %s, DataTypeOID: %d, Format: %d",
			i, fd.Name, fd.DataTypeOID, fd.Format)
		if fd.Format == 0 {
			fmt.Print(" (Text Format ✓)")
		} else {
			fmt.Print(" (Binary Format ✗)")
		}
		fmt.Println()
	}

	// Read data
	if rows.Next() {
		values, err := rows.Values()
		if err != nil {
			log.Fatalf("Failed to get values: %v", err)
		}
		fmt.Println("\nData values:")
		for i, v := range values {
			fmt.Printf("  [%d] Type: %T, Value: %v\n", i, v, v)
		}
	}
}
