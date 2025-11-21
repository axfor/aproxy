package sqlrewrite

import (
	"regexp"
	"strings"
)

// UnsupportedFeature represents a detected unsupported MySQL feature
type UnsupportedFeature struct {
	Feature     string // Feature name
	SQL         string // Original SQL containing the feature
	Suggestion  string // Suggested alternative
	Severity    string // "error", "warning", "info"
	Category    string // "syntax", "function", "type", "other"
}

// UnsupportedDetector detects unsupported MySQL features in SQL statements
type UnsupportedDetector struct {
	patterns []UnsupportedPattern
}

// UnsupportedPattern defines a pattern for detecting unsupported features
type UnsupportedPattern struct {
	Name       string
	Pattern    *regexp.Regexp
	Suggestion string
	Severity   string
	Category   string
}

// NewUnsupportedDetector creates a new unsupported feature detector
func NewUnsupportedDetector() *UnsupportedDetector {
	return &UnsupportedDetector{
		patterns: buildUnsupportedPatterns(),
	}
}

// Detect scans SQL for unsupported features
func (d *UnsupportedDetector) Detect(sql string) []UnsupportedFeature {
	var features []UnsupportedFeature
	upperSQL := strings.ToUpper(sql)

	for _, pattern := range d.patterns {
		if pattern.Pattern.MatchString(upperSQL) {
			features = append(features, UnsupportedFeature{
				Feature:    pattern.Name,
				SQL:        sql,
				Suggestion: pattern.Suggestion,
				Severity:   pattern.Severity,
				Category:   pattern.Category,
			})
		}
	}

	return features
}

// buildUnsupportedPatterns creates all unsupported feature detection patterns
func buildUnsupportedPatterns() []UnsupportedPattern {
	return []UnsupportedPattern{
		// SQL Syntax
		{
			Name:       "UPDATE ... LIMIT",
			Pattern:    regexp.MustCompile(`(?i)UPDATE\s+.*\s+LIMIT\s+\d+`),
			Suggestion: "Use subquery: UPDATE ... WHERE id IN (SELECT id ... LIMIT n)",
			Severity:   "error",
			Category:   "syntax",
		},
		{
			Name:       "DELETE ... LIMIT",
			Pattern:    regexp.MustCompile(`(?i)DELETE\s+.*\s+LIMIT\s+\d+`),
			Suggestion: "Use subquery: DELETE ... WHERE id IN (SELECT id ... LIMIT n)",
			Severity:   "error",
			Category:   "syntax",
		},
		{
			Name:       "STRAIGHT_JOIN",
			Pattern:    regexp.MustCompile(`(?i)STRAIGHT_JOIN`),
			Suggestion: "Use explicit JOIN order or pg_hint_plan extension",
			Severity:   "warning",
			Category:   "syntax",
		},
		{
			Name:       "FORCE INDEX",
			Pattern:    regexp.MustCompile(`(?i)FORCE\s+INDEX`),
			Suggestion: "Remove or use pg_hint_plan extension",
			Severity:   "warning",
			Category:   "syntax",
		},
		{
			Name:       "USE INDEX",
			Pattern:    regexp.MustCompile(`(?i)USE\s+INDEX`),
			Suggestion: "Remove or use pg_hint_plan extension",
			Severity:   "warning",
			Category:   "syntax",
		},
		{
			Name:       "IGNORE INDEX",
			Pattern:    regexp.MustCompile(`(?i)IGNORE\s+INDEX`),
			Suggestion: "Remove hint, let PostgreSQL optimizer decide",
			Severity:   "warning",
			Category:   "syntax",
		},
		{
			Name:       "INSERT DELAYED",
			Pattern:    regexp.MustCompile(`(?i)INSERT\s+DELAYED`),
			Suggestion: "Use regular INSERT (DELAYED is deprecated in MySQL 5.7+)",
			Severity:   "warning",
			Category:   "syntax",
		},
		{
			Name:       "PARTITION BY in CREATE TABLE",
			Pattern:    regexp.MustCompile(`(?i)PARTITION\s+BY`),
			Suggestion: "Rewrite using PostgreSQL declarative partitioning syntax",
			Severity:   "error",
			Category:   "syntax",
		},
		{
			Name:       "VALUES() function in INSERT ... ON DUPLICATE KEY UPDATE",
			Pattern:    regexp.MustCompile(`(?i)ON\s+DUPLICATE\s+KEY\s+UPDATE.*VALUES\s*\(`),
			Suggestion: "Use EXCLUDED table reference instead of VALUES()",
			Severity:   "error",
			Category:   "syntax",
		},

		// Functions
		{
			Name:       "FOUND_ROWS()",
			Pattern:    regexp.MustCompile(`(?i)FOUND_ROWS\s*\(\s*\)`),
			Suggestion: "Use COUNT(*) OVER() or separate COUNT query",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "GET_LOCK()",
			Pattern:    regexp.MustCompile(`(?i)GET_LOCK\s*\(`),
			Suggestion: "Use pg_advisory_lock(key::BIGINT)",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "RELEASE_LOCK()",
			Pattern:    regexp.MustCompile(`(?i)RELEASE_LOCK\s*\(`),
			Suggestion: "Use pg_advisory_unlock(key::BIGINT)",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "IS_FREE_LOCK()",
			Pattern:    regexp.MustCompile(`(?i)IS_FREE_LOCK\s*\(`),
			Suggestion: "Query pg_locks system view",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "DATE_FORMAT()",
			Pattern:    regexp.MustCompile(`(?i)DATE_FORMAT\s*\(`),
			Suggestion: "Use TO_CHAR(date, format) with PostgreSQL format strings",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "STR_TO_DATE()",
			Pattern:    regexp.MustCompile(`(?i)STR_TO_DATE\s*\(`),
			Suggestion: "Use TO_DATE() or TO_TIMESTAMP() with PostgreSQL format",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "TIMESTAMPDIFF()",
			Pattern:    regexp.MustCompile(`(?i)TIMESTAMPDIFF\s*\(`),
			Suggestion: "Use EXTRACT(EPOCH FROM (t2 - t1))",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "FORMAT()",
			Pattern:    regexp.MustCompile(`(?i)FORMAT\s*\(\s*\d`),
			Suggestion: "Use TO_CHAR(number, format)",
			Severity:   "warning",
			Category:   "function",
		},
		{
			Name:       "ENCRYPT()",
			Pattern:    regexp.MustCompile(`(?i)ENCRYPT\s*\(`),
			Suggestion: "Use pgcrypto extension functions",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "PASSWORD()",
			Pattern:    regexp.MustCompile(`(?i)PASSWORD\s*\(`),
			Suggestion: "Use crypt() or pgcrypto extension (PASSWORD is deprecated)",
			Severity:   "warning",
			Category:   "function",
		},
		{
			Name:       "INET_ATON()",
			Pattern:    regexp.MustCompile(`(?i)INET_ATON\s*\(`),
			Suggestion: "Use inet data type and casting",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "INET_NTOA()",
			Pattern:    regexp.MustCompile(`(?i)INET_NTOA\s*\(`),
			Suggestion: "Use inet data type and host() function",
			Severity:   "error",
			Category:   "function",
		},
		{
			Name:       "LOAD_FILE()",
			Pattern:    regexp.MustCompile(`(?i)LOAD_FILE\s*\(`),
			Suggestion: "Security risk - not supported in PostgreSQL",
			Severity:   "error",
			Category:   "function",
		},

		// Types (in CREATE TABLE context)
		{
			Name:       "SET data type",
			Pattern:    regexp.MustCompile(`(?i)CREATE\s+TABLE.*\s+SET\s*\(`),
			Suggestion: "Use TEXT[] array or separate many-to-many table",
			Severity:   "error",
			Category:   "type",
		},
		{
			Name:       "GEOMETRY/SPATIAL types",
			Pattern:    regexp.MustCompile(`(?i)(GEOMETRY|POINT|LINESTRING|POLYGON|MULTIPOINT|MULTILINESTRING|MULTIPOLYGON|GEOMETRYCOLLECTION)`),
			Suggestion: "Install and use PostGIS extension",
			Severity:   "error",
			Category:   "type",
		},

		// Other
		{
			Name:       "LOAD DATA INFILE",
			Pattern:    regexp.MustCompile(`(?i)LOAD\s+DATA\s+(LOCAL\s+)?INFILE`),
			Suggestion: "Use PostgreSQL COPY FROM command",
			Severity:   "error",
			Category:   "other",
		},
		{
			Name:       "LOCK TABLES",
			Pattern:    regexp.MustCompile(`(?i)LOCK\s+TABLES`),
			Suggestion: "Use transaction-level locks or LOCK TABLE (different syntax)",
			Severity:   "warning",
			Category:   "other",
		},
		{
			Name:       "UNLOCK TABLES",
			Pattern:    regexp.MustCompile(`(?i)UNLOCK\s+TABLES`),
			Suggestion: "Locks are released at transaction end in PostgreSQL",
			Severity:   "info",
			Category:   "other",
		},
		{
			Name:       "User variable (@variable)",
			Pattern:    regexp.MustCompile(`@\w+`),
			Suggestion: "Use temporary tables or session variables",
			Severity:   "warning",
			Category:   "other",
		},
	}
}
