package mapper

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
)

func TestErrorMapper_GetMySQLErrorCode(t *testing.T) {
	em := NewErrorMapper()

	tests := []struct {
		name      string
		sqlState  string
		expected  uint16
	}{
		{"duplicate key", "23505", ER_DUP_ENTRY},
		{"foreign key violation", "23503", ER_NO_REFERENCED_ROW_2},
		{"access denied", "42501", ER_TABLEACCESS_DENIED_ERROR},
		{"table not found", "42P01", ER_NO_SUCH_TABLE},
		{"column not found", "42703", ER_BAD_FIELD_ERROR},
		{"syntax error", "42601", ER_PARSE_ERROR},
		{"deadlock", "40P01", ER_LOCK_DEADLOCK},
		{"query interrupted", "57014", ER_QUERY_INTERRUPTED},
		{"unknown error", "99999", ER_UNKNOWN_ERROR},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := em.GetMySQLErrorCode(tt.sqlState)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestErrorMapper_MapError(t *testing.T) {
	em := NewErrorMapper()

	tests := []struct {
		name         string
		pgErr        error
		expectedCode uint16
		expectedMsg  string
	}{
		{
			name: "nil error",
			pgErr: nil,
			expectedCode: 0,
			expectedMsg: "",
		},
		{
			name: "duplicate key error",
			pgErr: &pgconn.PgError{
				Code:    "23505",
				Message: "duplicate key value violates unique constraint",
			},
			expectedCode: ER_DUP_ENTRY,
			expectedMsg:  "duplicate key value violates unique constraint",
		},
		{
			name: "table not found",
			pgErr: &pgconn.PgError{
				Code:    "42P01",
				Message: "relation does not exist",
			},
			expectedCode: ER_NO_SUCH_TABLE,
			expectedMsg:  "relation does not exist",
		},
		{
			name:         "generic error",
			pgErr:        errors.New("some error"),
			expectedCode: ER_UNKNOWN_ERROR,
			expectedMsg:  "some error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, msg := em.MapError(tt.pgErr)
			assert.Equal(t, tt.expectedCode, code)
			if tt.pgErr != nil {
				assert.Equal(t, tt.expectedMsg, msg)
			}
		})
	}
}

func BenchmarkErrorMapper_MapError(b *testing.B) {
	em := NewErrorMapper()
	pgErr := &pgconn.PgError{
		Code:    "23505",
		Message: "duplicate key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.MapError(pgErr)
	}
}
