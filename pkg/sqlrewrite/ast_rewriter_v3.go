package sqlrewrite

import (
	"fmt"
	"os"

	"github.com/pingcap/tidb/pkg/parser"
)

// ASTRewriter AST-based SQL rewriter
// This is the main class that integrates TypeMapper, ASTVisitor and PGGenerator
type ASTRewriter struct {
	parser    *parser.Parser
	visitor   *ASTVisitor
	generator *PGGenerator
	enabled   bool
}

// NewASTRewriter creates a new AST rewriter
func NewASTRewriter() *ASTRewriter {
	return &ASTRewriter{
		parser:    parser.New(),
		visitor:   NewASTVisitor(),
		generator: NewPGGenerator(),
		enabled:   true,
	}
}

// Rewrite rewrites MySQL SQL to PostgreSQL SQL
// This is the main public API
func (r *ASTRewriter) Rewrite(sql string) (string, error) {
	if !r.enabled {
		return sql, nil
	}

	// Step 1: Parse MySQL SQL to AST
	stmts, _, err := r.parser.Parse(sql, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to parse SQL: %w", err)
	}

	if len(stmts) == 0 {
		return "", fmt.Errorf("no statements found in SQL")
	}

	// Currently only handles single statement
	stmt := stmts[0]

	// Step 2: Traverse and transform AST
	// Reset visitor state
	r.visitor.ResetPlaceholders()

	// Use visitor to traverse and transform AST
	stmt.Accept(r.visitor)

	if err := r.visitor.GetError(); err != nil {
		return "", fmt.Errorf("AST transformation failed: %w", err)
	}

	// Step 3: Generate PostgreSQL SQL from transformed AST
	pgSQL, paramCount, err := r.generator.GenerateWithPlaceholders(stmt)
	if err != nil {
		return "", fmt.Errorf("SQL generation failed: %w", err)
	}

	// Step 4: Post-processing
	pgSQLBeforePost := pgSQL
	pgSQL = r.generator.PostProcess(pgSQL)

	// DEBUG: Log post-process changes
	if pgSQL != pgSQLBeforePost {
		fmt.Fprintf(os.Stderr, "PostProcess changed SQL: %q -> %q\n", pgSQLBeforePost, pgSQL)
	}

	// Record placeholder count (for debugging)
	_ = paramCount

	return pgSQL, nil
}

// RewriteBatch rewrites multiple SQL statements in batch
func (r *ASTRewriter) RewriteBatch(sqls []string) ([]string, error) {
	results := make([]string, len(sqls))

	for i, sql := range sqls {
		rewritten, err := r.Rewrite(sql)
		if err != nil {
			return nil, fmt.Errorf("failed to rewrite statement %d: %w", i, err)
		}
		results[i] = rewritten
	}

	return results, nil
}

// Enable activates the AST rewriter
func (r *ASTRewriter) Enable() {
	r.enabled = true
}

// Disable deactivates the AST rewriter (will return original SQL directly)
func (r *ASTRewriter) Disable() {
	r.enabled = false
}

// IsEnabled checks if the AST rewriter is enabled
func (r *ASTRewriter) IsEnabled() bool {
	return r.enabled
}
