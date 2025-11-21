package sqlrewrite

import (
	"fmt"
	"os"
	"strings"
)

// Rewriter is the main SQL rewriter using AST-based rewriting
type Rewriter struct {
	enabled     bool
	astRewriter *ASTRewriter
}

// NewRewriter creates a rewriter with AST rewriter
func NewRewriter(enabled bool) *Rewriter {
	return &Rewriter{
		enabled:     enabled,
		astRewriter: NewASTRewriter(),
	}
}

// Rewrite rewrites a MySQL SQL statement to PostgreSQL using AST rewriter
func (r *Rewriter) Rewrite(sql string) (string, error) {
	if !r.enabled {
		return sql, nil
	}

	sql = strings.TrimSpace(sql)

	// Use AST rewriter
	if r.astRewriter != nil {
		rewritten, err := r.astRewriter.Rewrite(sql)
		if err == nil {
			return rewritten, nil
		}
		// Log error and return original SQL
		fmt.Fprintf(os.Stderr, "AST rewriter failed: %v\n", err)
		return sql, err
	}

	return sql, nil
}

// RewritePrepared rewrites a prepared statement and returns the parameter count
func (r *Rewriter) RewritePrepared(sql string) (string, int, error) {
	rewritten, err := r.Rewrite(sql)
	if err != nil {
		return "", 0, err
	}

	// Count placeholders ($ followed by digits)
	paramCount := 0
	for i := 0; i < len(rewritten); i++ {
		if rewritten[i] == '$' && i+1 < len(rewritten) && rewritten[i+1] >= '0' && rewritten[i+1] <= '9' {
			// Found a parameter placeholder
			// Parse the number to get the highest parameter index
			j := i + 1
			num := 0
			for j < len(rewritten) && rewritten[j] >= '0' && rewritten[j] <= '9' {
				num = num*10 + int(rewritten[j]-'0')
				j++
			}
			if num > paramCount {
				paramCount = num
			}
		}
	}

	return rewritten, paramCount, nil
}

// Helper methods for statement type checking

func (r *Rewriter) IsShowStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upperSQL, "SHOW ") ||
		strings.HasPrefix(upperSQL, "DESCRIBE ") ||
		strings.HasPrefix(upperSQL, "DESC ")
}

func (r *Rewriter) IsSetStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upperSQL, "SET ")
}

func (r *Rewriter) IsUseStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upperSQL, "USE ")
}

func (r *Rewriter) IsBeginStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return upperSQL == "BEGIN" ||
		upperSQL == "START TRANSACTION" ||
		strings.HasPrefix(upperSQL, "BEGIN ") ||
		strings.HasPrefix(upperSQL, "START TRANSACTION ")
}

func (r *Rewriter) IsCommitStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return upperSQL == "COMMIT" ||
		strings.HasPrefix(upperSQL, "COMMIT ")
}

func (r *Rewriter) IsRollbackStatement(sql string) bool {
	upperSQL := strings.ToUpper(strings.TrimSpace(sql))
	return upperSQL == "ROLLBACK" ||
		strings.HasPrefix(upperSQL, "ROLLBACK ")
}
