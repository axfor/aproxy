package main

import (
	"fmt"
	"github.com/go-mysql-org/go-mysql/mysql"
)

func main() {
	// Test creating a simple Resultset and dumping field packets
	names := []string{"id", "name", "price"}

	values := [][]interface{}{
		{int64(1), "Product 1", "99.99"}, // DECIMAL as string
	}

	// Build Resultset manually (simulating our approach)
	resultset := &mysql.Resultset{
		Fields:     make([]*mysql.Field, len(names)),
		FieldNames: make(map[string]int, len(names)),
		RowDatas:   make([]mysql.RowData, 0, len(values)),
	}

	// Field 0: id (INT)
	resultset.Fields[0] = &mysql.Field{
		Name:         []byte("id"),
		Type:         mysql.MYSQL_TYPE_LONGLONG,
		Charset:      63,
		Flag:         mysql.BINARY_FLAG,
		ColumnLength: 20,
	}
	resultset.FieldNames["id"] = 0

	// Field 1: name (VARCHAR)
	resultset.Fields[1] = &mysql.Field{
		Name:         []byte("name"),
		Type:         mysql.MYSQL_TYPE_VAR_STRING,
		Charset:      33,
		ColumnLength: 255,
	}
	resultset.FieldNames["name"] = 1

	// Field 2: price (VARCHAR - converted from DECIMAL)
	resultset.Fields[2] = &mysql.Field{
		Name:         []byte("price"),
		Type:         mysql.MYSQL_TYPE_VAR_STRING,
		Charset:      33,
		ColumnLength: 255,
	}
	resultset.FieldNames["price"] = 2

	// Build RowDatas
	for _, row := range values {
		var rowData []byte
		for _, value := range row {
			b, err := mysql.FormatTextValue(value)
			if err != nil {
				fmt.Printf("ERROR formatting value: %v\n", err)
				return
			}

			if b == nil {
				rowData = append(rowData, 0xfb)
			} else {
				rowData = append(rowData, mysql.PutLengthEncodedString(b)...)
			}
		}
		resultset.RowDatas = append(resultset.RowDatas, rowData)
	}

	fmt.Println("=== Field Definitions ===")
	for i, field := range resultset.Fields {
		fmt.Printf("Field %d: %s\n", i, field.Name)
		fmt.Printf("  Type: %d\n", field.Type)
		fmt.Printf("  Charset: %d\n", field.Charset)
		fmt.Printf("  ColumnLength: %d\n", field.ColumnLength)
		fmt.Printf("  Flag: %d\n", field.Flag)
	}

	fmt.Println("\n=== Field Packets (Hex Dump) ===")
	for i, field := range resultset.Fields {
		packet := field.Dump()
		fmt.Printf("Field %d packet (len=%d): % X\n", i, len(packet), packet)
	}

	fmt.Println("\n=== RowData (Hex Dump) ===")
	for i, rowData := range resultset.RowDatas {
		fmt.Printf("Row %d (len=%d): % X\n", i, len(rowData), rowData)
	}

	fmt.Println("\n=== FieldNames Map ===")
	fmt.Printf("%v\n", resultset.FieldNames)
}
