package main

import (
	"fmt"
	"github.com/go-mysql-org/go-mysql/mysql"
)

func main() {
	// Test the BuildSimpleTextResultset with test data matching TestInsertAndSelect
	names := []string{"id", "name", "price"}

	values := [][]interface{}{
		{int64(1), "Product 1", float64(99.99)},
	}

	fmt.Println("=== Testing BuildSimpleTextResultset ===")
	fmt.Printf("Input names: %v\n", names)
	fmt.Printf("Input values: %v\n", values)

	rs, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		fmt.Printf("ERROR: %v\n", err)
		return
	}

	fmt.Printf("\n=== Resultset Structure ===\n")
	fmt.Printf("Number of fields: %d\n", len(rs.Fields))
	fmt.Printf("Number of RowDatas: %d\n", len(rs.RowDatas))
	fmt.Printf("FieldNames map: %v\n", rs.FieldNames)

	fmt.Printf("\n=== Field Details ===\n")
	for i, field := range rs.Fields {
		fmt.Printf("Field %d:\n", i)
		fmt.Printf("  Name: %s\n", field.Name)
		fmt.Printf("  Type: %d\n", field.Type)
		fmt.Printf("  Charset: %d\n", field.Charset)
		fmt.Printf("  ColumnLength: %d\n", field.ColumnLength)
		fmt.Printf("  Flag: %d\n", field.Flag)
		fmt.Printf("  Decimal: %d\n", field.Decimal)
	}

	fmt.Printf("\n=== RowData Hex Dump ===\n")
	for i, rowData := range rs.RowDatas {
		fmt.Printf("Row %d (length %d): % X\n", i, len(rowData), rowData)
	}

	// Now test with ColumnLength set manually
	fmt.Printf("\n\n=== Testing with Manual ColumnLength ===\n")
	for i := range rs.Fields {
		if rs.Fields[i].ColumnLength == 0 {
			switch rs.Fields[i].Type {
			case mysql.MYSQL_TYPE_LONGLONG:
				rs.Fields[i].ColumnLength = 20
			case mysql.MYSQL_TYPE_VAR_STRING, mysql.MYSQL_TYPE_STRING:
				rs.Fields[i].ColumnLength = 255
			case mysql.MYSQL_TYPE_DOUBLE:
				rs.Fields[i].ColumnLength = 22
			default:
				rs.Fields[i].ColumnLength = 255
			}
		}
	}

	fmt.Printf("\n=== Field Details After ColumnLength Fix ===\n")
	for i, field := range rs.Fields {
		fmt.Printf("Field %d:\n", i)
		fmt.Printf("  Name: %s\n", field.Name)
		fmt.Printf("  Type: %d\n", field.Type)
		fmt.Printf("  Charset: %d\n", field.Charset)
		fmt.Printf("  ColumnLength: %d\n", field.ColumnLength)
		fmt.Printf("  Flag: %d\n", field.Flag)
	}

	// Test Field.Dump() to see the actual packet data
	fmt.Printf("\n=== Field Packet Dumps ===\n")
	for i, field := range rs.Fields {
		packetData := field.Dump()
		fmt.Printf("Field %d packet (length %d): % X\n", i, len(packetData), packetData)
	}
}
