package schema

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// syncMapTyped is a type-safe wrapper around sync.Map
// It provides generic type safety and eliminates runtime type assertions
type syncMapTyped[K comparable, V any] struct {
	m sync.Map
}

// Load returns the value stored in the map for a key, or zero value if not present
func (s *syncMapTyped[K, V]) Load(key K) (V, bool) {
	v, ok := s.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// Store sets the value for a key
func (s *syncMapTyped[K, V]) Store(key K, value V) {
	s.m.Store(key, value)
}

// Delete deletes the value for a key
func (s *syncMapTyped[K, V]) Delete(key K) {
	s.m.Delete(key)
}

// Range calls f sequentially for each key and value present in the map
func (s *syncMapTyped[K, V]) Range(f func(key K, value V) bool) {
	s.m.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

// TableInfo contains schema information for a table
type TableInfo struct {
	TableName      string
	AutoIncrColumn string    // Empty string if no auto-increment column
	LastRefreshed  time.Time // When this info was last queried
	TTL            time.Duration
}

// Cache is a global schema cache shared across all sessions
type Cache struct {
	tables *syncMapTyped[string, *TableInfo] // Type-safe map[string]*TableInfo
	ttl    time.Duration
	mu     sync.RWMutex
}

var (
	// GlobalCache is the singleton schema cache instance
	GlobalCache *Cache
	once        sync.Once
)

// InitGlobalCache initializes the global schema cache
func InitGlobalCache(ttl time.Duration) *Cache {
	once.Do(func() {
		GlobalCache = &Cache{
			tables: &syncMapTyped[string, *TableInfo]{},
			ttl:    ttl,
		}
	})
	return GlobalCache
}

// GetGlobalCache returns the global schema cache instance
func GetGlobalCache() *Cache {
	if GlobalCache == nil {
		return InitGlobalCache(5 * time.Minute) // Default 5 minutes TTL
	}
	return GlobalCache
}

// GetAutoIncrementColumn returns the AUTO_INCREMENT column name for a table
// It uses cached data if available and not expired, otherwise queries PostgreSQL
// The cache key format is "database.table" to support multiple databases
func (c *Cache) GetAutoIncrementColumn(conn *pgx.Conn, database, tableName string) string {
	// Build cache key as "database.table"
	cacheKey := database + "." + tableName

	// Try to get from cache (no type assertion needed with generics!)
	if tableInfo, ok := c.tables.Load(cacheKey); ok {
		// Check if cache is still valid
		if time.Since(tableInfo.LastRefreshed) < tableInfo.TTL {
			return tableInfo.AutoIncrColumn
		}
	}

	// Cache miss or expired, query from PostgreSQL
	columnName := c.queryAutoIncrementColumn(conn, tableName)

	// Update cache
	c.tables.Store(cacheKey, &TableInfo{
		TableName:      tableName,
		AutoIncrColumn: columnName,
		LastRefreshed:  time.Now(),
		TTL:            c.ttl,
	})

	return columnName
}

// queryAutoIncrementColumn queries PostgreSQL system tables to find auto-increment column
func (c *Cache) queryAutoIncrementColumn(conn *pgx.Conn, tableName string) string {
	if conn == nil {
		return ""
	}

	ctx := context.Background()

	// Query PostgreSQL information_schema to find SERIAL or IDENTITY columns
	// SERIAL columns have column_default like 'nextval(...)'
	// IDENTITY columns have is_identity = 'YES'
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_name = $1
		  AND table_schema = current_schema()
		  AND (
		      column_default LIKE 'nextval(%'
		      OR is_identity = 'YES'
		  )
		ORDER BY ordinal_position
		LIMIT 1
	`

	var columnName string
	err := conn.QueryRow(ctx, query, strings.ToLower(tableName)).Scan(&columnName)
	if err != nil {
		// No auto-increment column found or query failed
		return ""
	}

	return columnName
}

// InvalidateTable removes a table from the cache
// This should be called when a DDL statement modifies the table
// The key format is "database.table"
func (c *Cache) InvalidateTable(database, tableName string) {
	cacheKey := database + "." + tableName
	c.tables.Delete(cacheKey)
}

// InvalidateAll clears the entire cache
func (c *Cache) InvalidateAll() {
	c.tables.Range(func(key string, value *TableInfo) bool {
		c.tables.Delete(key)
		return true
	})
}

// RefreshTable forces a refresh of table schema information
func (c *Cache) RefreshTable(conn *pgx.Conn, database, tableName string) string {
	// Force refresh by invalidating first
	c.InvalidateTable(database, tableName)
	// Then query and cache
	return c.GetAutoIncrementColumn(conn, database, tableName)
}

// GetCacheStats returns statistics about the cache
func (c *Cache) GetCacheStats() map[string]interface{} {
	count := 0
	c.tables.Range(func(key string, value *TableInfo) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"cached_tables": count,
		"ttl_seconds":   c.ttl.Seconds(),
	}
}

// SetTTL updates the TTL for new cache entries
func (c *Cache) SetTTL(ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ttl = ttl
}

// StartPeriodicRefresh starts a background goroutine that periodically refreshes expired entries
// This is optional and can be used to proactively refresh popular tables
func (c *Cache) StartPeriodicRefresh(interval time.Duration, conn *pgx.Conn) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			c.refreshExpiredEntries(conn)
		}
	}()
}

// refreshExpiredEntries refreshes all expired cache entries
func (c *Cache) refreshExpiredEntries(conn *pgx.Conn) {
	now := time.Now()
	var expiredKeys []string

	// Collect expired tables
	c.tables.Range(func(cacheKey string, info *TableInfo) bool {
		if now.Sub(info.LastRefreshed) >= info.TTL {
			expiredKeys = append(expiredKeys, cacheKey)
		}
		return true
	})

	// Refresh expired tables
	// Note: We can't refresh without knowing the database context
	// So we just invalidate expired entries and let them refresh on next access
	for _, cacheKey := range expiredKeys {
		c.tables.Delete(cacheKey)
	}
}
