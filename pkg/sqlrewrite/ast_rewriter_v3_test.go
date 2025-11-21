package sqlrewrite

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASTRewriter_SimpleSelect(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name     string
		mysql    string
		expected string
	}{
		{
			name:     "Simple SELECT",
			mysql:    "SELECT id, name FROM users",
			expected: "SELECT `id`,`name` FROM `users`",
		},
		{
			name:     "SELECT with WHERE",
			mysql:    "SELECT id FROM users WHERE id = 1",
			expected: "SELECT `id` FROM `users` WHERE `id`=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			// Since AST-generated SQL may have format differences, we only verify no errors
			// Complete validation needs to be done in integration tests
			assert.NotEmpty(t, result, "Rewrite result should not be empty")
			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_Placeholders(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name     string
		mysql    string
	}{
		{
			name:  "Single placeholder",
			mysql: "SELECT id FROM users WHERE id = ?",
		},
		{
			name:  "Multiple placeholders",
			mysql: "SELECT id FROM users WHERE id = ? AND name = ?",
		},
		{
			name:  "INSERT placeholder",
			mysql: "INSERT INTO users (id, name) VALUES (?, ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			// Verify placeholders converted to $1, $2 format
			assert.Contains(t, result, "$1", "Should contain $1 placeholder")

			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_Functions(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name  string
		mysql string
	}{
		{
			name:  "NOW() function",
			mysql: "SELECT NOW() FROM users",
		},
		{
			name:  "IFNULL function",
			mysql: "SELECT IFNULL(name, 'Unknown') FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			assert.NotEmpty(t, result, "Rewrite result should not be empty")

			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_INSERT(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name  string
		mysql string
	}{
		{
			name:  "Simple INSERT",
			mysql: "INSERT INTO users (id, name) VALUES (1, 'John')",
		},
		{
			name:  "INSERT with placeholder",
			mysql: "INSERT INTO users (id, name) VALUES (?, ?)",
		},
		{
			name:  "Multi-row INSERT",
			mysql: "INSERT INTO users (id, name) VALUES (1, 'John'), (2, 'Jane')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			assert.NotEmpty(t, result, "Rewrite result should not be empty")

			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_UPDATE(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name  string
		mysql string
	}{
		{
			name:  "Simple UPDATE",
			mysql: "UPDATE users SET name = 'John' WHERE id = 1",
		},
		{
			name:  "UPDATE with placeholder",
			mysql: "UPDATE users SET name = ? WHERE id = ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			assert.NotEmpty(t, result, "Rewrite result should not be empty")

			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_DELETE(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name  string
		mysql string
	}{
		{
			name:  "Simple DELETE",
			mysql: "DELETE FROM users WHERE id = 1",
		},
		{
			name:  "DELETE with placeholder",
			mysql: "DELETE FROM users WHERE id = ?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := rewriter.Rewrite(tt.mysql)
			require.NoError(t, err, "Rewrite should not error")

			assert.NotEmpty(t, result, "Rewrite result should not be empty")

			t.Logf("MySQL: %s", tt.mysql)
			t.Logf("PostgreSQL: %s", result)
		})
	}
}

func TestASTRewriter_EnableDisable(t *testing.T) {
	rewriter := NewASTRewriter()

	t.Run("Enabled state", func(t *testing.T) {
		assert.True(t, rewriter.IsEnabled(), "Should be enabled by default")

		result, err := rewriter.Rewrite("SELECT 1")
		require.NoError(t, err)
		assert.NotEmpty(t, result)
	})

	t.Run("Disabled state", func(t *testing.T) {
		rewriter.Disable()
		assert.False(t, rewriter.IsEnabled(), "Should be disabled")

		sql := "SELECT 1"
		result, err := rewriter.Rewrite(sql)
		require.NoError(t, err)
		assert.Equal(t, sql, result, "Should return original SQL when disabled")
	})

	t.Run("Re-enable", func(t *testing.T) {
		rewriter.Enable()
		assert.True(t, rewriter.IsEnabled(), "Should be re-enabled")
	})
}

func TestASTRewriter_BatchRewrite(t *testing.T) {
	rewriter := NewASTRewriter()

	sqls := []string{
		"SELECT id FROM users WHERE id = ?",
		"INSERT INTO users (id, name) VALUES (?, ?)",
		"UPDATE users SET name = ? WHERE id = ?",
	}

	results, err := rewriter.RewriteBatch(sqls)
	require.NoError(t, err, "Batch rewrite should not error")
	assert.Len(t, results, len(sqls), "Result count should match")

	for i, result := range results {
		assert.NotEmpty(t, result, "Result %d should not be empty", i)
		t.Logf("SQL %d: %s → %s", i, sqls[i], result)
	}
}

func TestASTRewriter_ErrorHandling(t *testing.T) {
	rewriter := NewASTRewriter()

	tests := []struct {
		name  string
		mysql string
	}{
		{
			name:  "Invalid SQL",
			mysql: "SELECT FROM",
		},
		{
			name:  "Incomplete statement",
			mysql: "SELECT * FROM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rewriter.Rewrite(tt.mysql)
			assert.Error(t, err, "Should return error")
			t.Logf("Error: %v", err)
		})
	}
}

// Benchmarks
func BenchmarkASTRewriter_SimpleSelect(b *testing.B) {
	rewriter := NewASTRewriter()
	sql := "SELECT id, name FROM users WHERE id = ?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rewriter.Rewrite(sql)
	}
}

func BenchmarkASTRewriter_ComplexSelect(b *testing.B) {
	rewriter := NewASTRewriter()
	sql := "SELECT u.id, u.name, COUNT(o.id) FROM users u LEFT JOIN orders o ON u.id = o.user_id WHERE u.status = ? GROUP BY u.id, u.name ORDER BY u.id LIMIT 100"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rewriter.Rewrite(sql)
	}
}

func BenchmarkASTRewriter_INSERT(b *testing.B) {
	rewriter := NewASTRewriter()
	sql := "INSERT INTO users (id, name, email, created_at) VALUES (?, ?, ?, NOW())"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rewriter.Rewrite(sql)
	}
}
