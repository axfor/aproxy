package mapper

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type ShowEmulator struct{}

func NewShowEmulator() *ShowEmulator {
	return &ShowEmulator{}
}

func (se *ShowEmulator) HandleShowCommand(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))

	if strings.HasPrefix(upperSQL, "SHOW DATABASES") {
		return se.showDatabases(ctx, conn)
	}

	if strings.HasPrefix(upperSQL, "SHOW TABLES") {
		return se.showTables(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "SHOW COLUMNS") || strings.HasPrefix(upperSQL, "SHOW FIELDS") {
		return se.showColumns(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "DESCRIBE ") || strings.HasPrefix(upperSQL, "DESC ") {
		return se.describe(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "SHOW CREATE TABLE") {
		return se.showCreateTable(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "SHOW INDEX") {
		return se.showIndex(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "SHOW STATUS") {
		return se.showStatus(ctx, conn)
	}

	if strings.HasPrefix(upperSQL, "SHOW VARIABLES") {
		return se.showVariables(ctx, conn, sql)
	}

	if strings.HasPrefix(upperSQL, "SHOW WARNINGS") {
		return se.showWarnings(ctx, conn)
	}

	return nil, fmt.Errorf("unsupported SHOW command: %s", sql)
}

func (se *ShowEmulator) showDatabases(ctx context.Context, conn *pgx.Conn) (pgx.Rows, error) {
	query := `
		SELECT schema_name AS "Database"
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name
	`
	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showTables(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	var schemaName string

	upperSQL := strings.ToUpper(sql)
	if strings.Contains(upperSQL, "FROM") || strings.Contains(upperSQL, "IN") {
		parts := strings.Fields(sql)
		for i, part := range parts {
			if (strings.ToUpper(part) == "FROM" || strings.ToUpper(part) == "IN") && i+1 < len(parts) {
				schemaName = strings.Trim(parts[i+1], "`\"';")
				break
			}
		}
	}

	var query string
	if schemaName != "" {
		query = fmt.Sprintf(`
			SELECT table_name AS "Tables_in_%s"
			FROM information_schema.tables
			WHERE table_schema = '%s' AND table_type = 'BASE TABLE'
			ORDER BY table_name
		`, schemaName, schemaName)
	} else {
		query = `
			SELECT table_name AS "Tables"
			FROM information_schema.tables
			WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
			ORDER BY table_name
		`
	}

	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showColumns(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	tableName := se.extractTableName(sql)
	if tableName == "" {
		return nil, fmt.Errorf("table name not found in: %s", sql)
	}

	query := fmt.Sprintf(`
		SELECT
			column_name AS "Field",
			data_type AS "Type",
			is_nullable AS "Null",
			column_default AS "Default",
			'' AS "Key",
			'' AS "Extra"
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = '%s'
		ORDER BY ordinal_position
	`, tableName)

	return conn.Query(ctx, query)
}

func (se *ShowEmulator) describe(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	parts := strings.Fields(sql)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid DESCRIBE command: %s", sql)
	}

	tableName := strings.Trim(parts[1], "`\"';")

	query := fmt.Sprintf(`
		SELECT
			column_name AS "Field",
			data_type AS "Type",
			is_nullable AS "Null",
			column_default AS "Default",
			CASE
				WHEN EXISTS (
					SELECT 1 FROM information_schema.key_column_usage kcu
					WHERE kcu.table_schema = current_schema()
					  AND kcu.table_name = c.table_name
					  AND kcu.column_name = c.column_name
				) THEN 'PRI'
				ELSE ''
			END AS "Key",
			CASE
				WHEN column_default LIKE 'nextval%%' THEN 'auto_increment'
				ELSE ''
			END AS "Extra"
		FROM information_schema.columns c
		WHERE c.table_schema = current_schema()
		  AND c.table_name = '%s'
		ORDER BY c.ordinal_position
	`, tableName)

	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showCreateTable(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	parts := strings.Fields(sql)
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid SHOW CREATE TABLE command: %s", sql)
	}

	tableName := strings.Trim(parts[3], "`\"';")

	query := fmt.Sprintf(`
		SELECT
			'%s' AS "Table",
			'CREATE TABLE %s (...) ' AS "Create Table"
	`, tableName, tableName)

	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showIndex(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	tableName := se.extractTableName(sql)
	if tableName == "" {
		return nil, fmt.Errorf("table name not found in: %s", sql)
	}

	query := fmt.Sprintf(`
		SELECT
			'%s' AS "Table",
			0 AS "Non_unique",
			i.indexname AS "Key_name",
			1 AS "Seq_in_index",
			'' AS "Column_name",
			'A' AS "Collation",
			0 AS "Cardinality",
			NULL AS "Sub_part",
			NULL AS "Packed",
			'YES' AS "Null",
			'BTREE' AS "Index_type",
			'' AS "Comment"
		FROM pg_indexes i
		WHERE i.schemaname = current_schema()
		  AND i.tablename = '%s'
	`, tableName, tableName)

	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showStatus(ctx context.Context, conn *pgx.Conn) (pgx.Rows, error) {
	query := `
		SELECT 'Uptime' AS "Variable_name", '0' AS "Value"
		UNION ALL
		SELECT 'Threads_connected', '1'
		UNION ALL
		SELECT 'Questions', '0'
		UNION ALL
		SELECT 'Slow_queries', '0'
	`
	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showVariables(ctx context.Context, conn *pgx.Conn, sql string) (pgx.Rows, error) {
	upperSQL := strings.ToUpper(sql)

	if strings.Contains(upperSQL, "LIKE") {
		parts := strings.Split(sql, "LIKE")
		if len(parts) > 1 {
			pattern := strings.TrimSpace(parts[1])
			pattern = strings.Trim(pattern, "'\"")
			pattern = strings.ReplaceAll(pattern, "%", "%%")

			query := fmt.Sprintf(`
				SELECT name AS "Variable_name", setting AS "Value"
				FROM pg_settings
				WHERE name LIKE '%s'
				ORDER BY name
			`, pattern)
			return conn.Query(ctx, query)
		}
	}

	query := `
		SELECT
			'version' AS "Variable_name",
			version() AS "Value"
		UNION ALL
		SELECT 'character_set_client', 'utf8mb4'
		UNION ALL
		SELECT 'character_set_connection', 'utf8mb4'
		UNION ALL
		SELECT 'character_set_results', 'utf8mb4'
		UNION ALL
		SELECT 'character_set_server', 'utf8mb4'
		UNION ALL
		SELECT 'collation_connection', 'utf8mb4_general_ci'
		UNION ALL
		SELECT 'collation_server', 'utf8mb4_general_ci'
		UNION ALL
		SELECT 'autocommit', 'ON'
		UNION ALL
		SELECT 'max_allowed_packet', '67108864'
		UNION ALL
		SELECT 'sql_mode', 'TRADITIONAL'
	`
	return conn.Query(ctx, query)
}

func (se *ShowEmulator) showWarnings(ctx context.Context, conn *pgx.Conn) (pgx.Rows, error) {
	query := `
		SELECT
			'Warning' AS "Level",
			0 AS "Code",
			'' AS "Message"
		LIMIT 0
	`
	return conn.Query(ctx, query)
}

func (se *ShowEmulator) extractTableName(sql string) string {
	upperSQL := strings.ToUpper(sql)

	keywords := []string{"FROM", "IN", "COLUMNS", "FIELDS", "INDEX"}
	for _, keyword := range keywords {
		if idx := strings.Index(upperSQL, keyword); idx != -1 {
			remaining := sql[idx+len(keyword):]
			parts := strings.Fields(remaining)
			if len(parts) > 0 {
				tableName := strings.Trim(parts[0], "`\"';")
				return tableName
			}
		}
	}

	parts := strings.Fields(sql)
	if len(parts) >= 2 {
		return strings.Trim(parts[len(parts)-1], "`\"';")
	}

	return ""
}

func (se *ShowEmulator) HandleSetCommand(ctx context.Context, sql string, sessionVars map[string]interface{}) error {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))

	if !strings.HasPrefix(upperSQL, "SET ") {
		return fmt.Errorf("not a SET command: %s", sql)
	}

	assignment := strings.TrimPrefix(sql, "SET ")
	assignment = strings.TrimPrefix(assignment, "set ")

	parts := strings.SplitN(assignment, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid SET syntax: %s", sql)
	}

	varName := strings.TrimSpace(parts[0])
	varValue := strings.TrimSpace(parts[1])
	varValue = strings.Trim(varValue, "'\"")

	varName = strings.TrimPrefix(varName, "@@")
	varName = strings.TrimPrefix(varName, "SESSION.")
	varName = strings.TrimPrefix(varName, "session.")

	if strings.HasPrefix(varName, "@") {
		sessionVars[varName] = varValue
		return nil
	}

	sessionVars[varName] = varValue
	return nil
}

func (se *ShowEmulator) HandleUseCommand(ctx context.Context, conn *pgx.Conn, sql string) error {
	parts := strings.Fields(sql)
	if len(parts) < 2 {
		return fmt.Errorf("invalid USE command: %s", sql)
	}

	dbName := strings.Trim(parts[1], "`\"';")

	_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s", dbName))
	return err
}
