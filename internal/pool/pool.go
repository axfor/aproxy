package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectionMode string

const (
	ModeSessionAffinity ConnectionMode = "session_affinity"
	ModePooled          ConnectionMode = "pooled"
	ModeHybrid          ConnectionMode = "hybrid"
)

type Config struct {
	Host        string
	Port        int
	Database    string
	User        string
	Password    string
	SSLMode     string
	MaxPoolSize int
	Mode        ConnectionMode
}

type Pool struct {
	config *Config
	pool   *pgxpool.Pool
	mode   ConnectionMode

	sessionConns map[string]*pgx.Conn
	mu           sync.RWMutex
}

func NewPool(cfg *Config) (*Pool, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxPoolSize)
	poolConfig.MinConns = 1
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	// Force Text Format for all fields to avoid Binary Format issues with DECIMAL/NUMERIC
	// Binary Format (Format=1) causes "busy buffer" errors because BuildSimpleTextResultset
	// expects Text Format (Format=0) data. Simple Query Protocol always uses Text Format.
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Set PostgreSQL timezone to system local timezone
	// This ensures LOCALTIMESTAMP and timestamp values match the system timezone
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// Get local timezone offset hours
		_, offset := time.Now().Zone()
		offsetHours := offset / 3600
		// Use numeric timezone like '+08' or '-05'
		tz := fmt.Sprintf("%+03d", offsetHours)
		_, err := conn.Exec(ctx, "SET timezone = '"+tz+"'")
		return err
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	return &Pool{
		config:       cfg,
		pool:         pool,
		mode:         cfg.Mode,
		sessionConns: make(map[string]*pgx.Conn),
	}, nil
}

func (p *Pool) AcquireForSession(ctx context.Context, sessionID string) (*pgx.Conn, error) {
	if p.mode == ModeSessionAffinity || p.mode == ModeHybrid {
		p.mu.Lock()
		defer p.mu.Unlock()

		if conn, exists := p.sessionConns[sessionID]; exists {
			return conn, nil
		}

		connString := fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			p.config.User,
			p.config.Password,
			p.config.Host,
			p.config.Port,
			p.config.Database,
			p.config.SSLMode,
		)

		// Parse and configure connection to use Simple Query Protocol (Text Format)
		connConfig, err := pgx.ParseConfig(connString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse connection string: %w", err)
		}
		connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

		conn, err := pgx.ConnectConfig(ctx, connConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create dedicated connection: %w", err)
		}

		// Set timezone to match system local timezone
		_, offset := time.Now().Zone()
		offsetHours := offset / 3600
		tz := fmt.Sprintf("%+03d", offsetHours)
		if _, err := conn.Exec(ctx, "SET timezone = '"+tz+"'"); err != nil {
			conn.Close(ctx)
			return nil, fmt.Errorf("failed to set timezone: %w", err)
		}

		p.sessionConns[sessionID] = conn
		return conn, nil
	}

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire connection from pool: %w", err)
	}

	return conn.Conn(), nil
}

func (p *Pool) ReleaseForSession(sessionID string) error {
	if p.mode == ModeSessionAffinity || p.mode == ModeHybrid {
		p.mu.Lock()
		defer p.mu.Unlock()

		if conn, exists := p.sessionConns[sessionID]; exists {
			ctx := context.Background()
			err := conn.Close(ctx)
			delete(p.sessionConns, sessionID)
			return err
		}
	}

	return nil
}

func (p *Pool) Stat() *pgxpool.Stat {
	return p.pool.Stat()
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx := context.Background()
	for sessionID, conn := range p.sessionConns {
		conn.Close(ctx)
		delete(p.sessionConns, sessionID)
	}

	p.pool.Close()
}

func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

func (p *Pool) GetSessionConnectionCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.sessionConns)
}
