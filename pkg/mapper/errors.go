package mapper

import (
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ER_DUP_ENTRY                  = 1062
	ER_NO_REFERENCED_ROW_2        = 1452
	ER_TABLEACCESS_DENIED_ERROR   = 1142
	ER_NO_SUCH_TABLE              = 1146
	ER_BAD_FIELD_ERROR            = 1054
	ER_PARSE_ERROR                = 1064
	ER_LOCK_DEADLOCK              = 1213
	ER_QUERY_INTERRUPTED          = 1317
	ER_ACCESS_DENIED_ERROR        = 1045
	ER_BAD_DB_ERROR               = 1049
	ER_DBACCESS_DENIED_ERROR      = 1044
	ER_SYNTAX_ERROR               = 1149
	ER_UNKNOWN_ERROR              = 1105
	ER_LOCK_WAIT_TIMEOUT          = 1205
	ER_TOO_MANY_USER_CONNECTIONS  = 1203
	ER_CON_COUNT_ERROR            = 1040
	ER_DATA_TOO_LONG              = 1406
	ER_BAD_NULL_ERROR             = 1048
	ER_DUP_KEYNAME                = 1061
	ER_NON_UNIQ_ERROR             = 1052
	ER_DIVISION_BY_ZERO           = 1365
	ER_TRUNCATED_WRONG_VALUE      = 1292
	ER_WARN_DATA_OUT_OF_RANGE     = 1264
	ER_NO_DEFAULT_FOR_FIELD       = 1364
	ER_ROW_IS_REFERENCED_2        = 1451
	ER_CHECK_CONSTRAINT_VIOLATED  = 3819
	ER_DISK_FULL                  = 1021
	ER_OUT_OF_RESOURCES           = 1041
	ER_SPECIFIC_ACCESS_DENIED     = 1227
	ER_LOCK_DEADLOCK_DETECTED     = 1213
)

type ErrorMapper struct {
	sqlStateToMySQL map[string]uint16
}

func NewErrorMapper() *ErrorMapper {
	return &ErrorMapper{
		sqlStateToMySQL: initSQLStateMapping(),
	}
}

func initSQLStateMapping() map[string]uint16 {
	return map[string]uint16{
		"23505": ER_DUP_ENTRY,
		"23503": ER_NO_REFERENCED_ROW_2,
		"42501": ER_TABLEACCESS_DENIED_ERROR,
		"42P01": ER_NO_SUCH_TABLE,
		"42703": ER_BAD_FIELD_ERROR,
		"42601": ER_PARSE_ERROR,
		"40P01": ER_LOCK_DEADLOCK,
		"57014": ER_QUERY_INTERRUPTED,
		"28000": ER_ACCESS_DENIED_ERROR,
		"3D000": ER_BAD_DB_ERROR,
		"42000": ER_DBACCESS_DENIED_ERROR,
		"42P02": ER_NO_SUCH_TABLE,
		"42P10": ER_PARSE_ERROR,
		"42704": ER_BAD_FIELD_ERROR,
		"42846": ER_PARSE_ERROR,
		"42883": ER_PARSE_ERROR,
		"55P03": ER_LOCK_WAIT_TIMEOUT,
		"53300": ER_TOO_MANY_USER_CONNECTIONS,
		"53400": ER_CON_COUNT_ERROR,
		"22001": ER_DATA_TOO_LONG,
		"23502": ER_BAD_NULL_ERROR,
		"42710": ER_DUP_KEYNAME,
		"42712": ER_DUP_KEYNAME,
		"23514": ER_CHECK_CONSTRAINT_VIOLATED,
		"53100": ER_DISK_FULL,
		"53200": ER_OUT_OF_RESOURCES,
		"42939": ER_TABLEACCESS_DENIED_ERROR,
		"40001": ER_LOCK_DEADLOCK_DETECTED,
		"23000": ER_DUP_ENTRY,
		"22003": ER_WARN_DATA_OUT_OF_RANGE,
		"22012": ER_DIVISION_BY_ZERO,
		"22007": ER_TRUNCATED_WRONG_VALUE,
		"22008": ER_TRUNCATED_WRONG_VALUE,
		"23001": ER_NO_DEFAULT_FOR_FIELD,
	}
}

func (em *ErrorMapper) MapError(pgErr error) (uint16, string) {
	if pgErr == nil {
		return 0, ""
	}

	if pge, ok := pgErr.(*pgconn.PgError); ok {
		if mysqlCode, exists := em.sqlStateToMySQL[pge.Code]; exists {
			return mysqlCode, pge.Message
		}

		return ER_UNKNOWN_ERROR, pge.Message
	}

	return ER_UNKNOWN_ERROR, pgErr.Error()
}

func (em *ErrorMapper) GetMySQLErrorCode(sqlState string) uint16 {
	if code, exists := em.sqlStateToMySQL[sqlState]; exists {
		return code
	}
	return ER_UNKNOWN_ERROR
}
