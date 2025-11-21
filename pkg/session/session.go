package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Session struct {
	ID            string
	User          string
	Database      string
	Charset       string
	Autocommit    bool
	InTransaction bool
	LastInsertID  uint64
	CreatedAt     time.Time
	LastActiveAt  time.Time
	ClientAddr    string

	sessionVars   map[string]interface{}
	userVars      map[string]interface{}
	preparedStmts map[uint32]*PreparedStatement

	pgConn *pgx.Conn
	mu     sync.RWMutex
}

type PreparedStatement struct {
	ID            uint32
	SQL           string
	OriginalSQL   string
	PGName        string
	ParamCount    int
	ParamTypes    []int
	ColumnCount   int
	ColumnTypes   []int
	ColumnNames   []string
}

type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func NewSession(user, database, clientAddr string) *Session {
	return &Session{
		ID:            uuid.New().String(),
		User:          user,
		Database:      database,
		Charset:       "utf8mb4",
		Autocommit:    true,
		InTransaction: false,
		LastInsertID:  0,
		CreatedAt:     time.Now(),
		LastActiveAt:  time.Now(),
		ClientAddr:    clientAddr,
		sessionVars:   make(map[string]interface{}),
		userVars:      make(map[string]interface{}),
		preparedStmts: make(map[uint32]*PreparedStatement),
	}
}

func (m *Manager) AddSession(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
}

func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	return s, ok
}

func (m *Manager) RemoveSession(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.Close()
		delete(m.sessions, id)
	}
}

func (m *Manager) GetAllSessions() []*Session {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	return sessions
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (s *Session) SetPGConn(conn *pgx.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pgConn = conn
}

func (s *Session) GetPGConn() *pgx.Conn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pgConn
}

func (s *Session) SetSessionVar(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionVars[key] = value
}

func (s *Session) GetSessionVar(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.sessionVars[key]
	return val, ok
}

func (s *Session) SetUserVar(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userVars[key] = value
}

func (s *Session) GetUserVar(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.userVars[key]
	return val, ok
}

func (s *Session) AddPreparedStatement(stmt *PreparedStatement) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preparedStmts[stmt.ID] = stmt
}

func (s *Session) GetPreparedStatement(id uint32) (*PreparedStatement, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stmt, ok := s.preparedStmts[id]
	return stmt, ok
}

func (s *Session) RemovePreparedStatement(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.preparedStmts, id)
}

func (s *Session) GetPreparedStatementCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.preparedStmts)
}

func (s *Session) UpdateLastActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActiveAt = time.Now()
}

func (s *Session) SetLastInsertID(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastInsertID = id
}

func (s *Session) GetLastInsertID() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastInsertID
}

func (s *Session) SetAutocommit(autocommit bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Autocommit == autocommit {
		return nil
	}

	s.Autocommit = autocommit

	if s.pgConn == nil {
		return nil
	}

	ctx := context.Background()
	if autocommit && s.InTransaction {
		_, err := s.pgConn.Exec(ctx, "COMMIT")
		if err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		s.InTransaction = false
	} else if !autocommit && !s.InTransaction {
		_, err := s.pgConn.Exec(ctx, "BEGIN")
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		s.InTransaction = true
	}

	return nil
}

func (s *Session) BeginTransaction() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.InTransaction {
		return nil
	}

	if s.pgConn == nil {
		return fmt.Errorf("no PostgreSQL connection")
	}

	ctx := context.Background()
	_, err := s.pgConn.Exec(ctx, "BEGIN")
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	s.InTransaction = true
	return nil
}

func (s *Session) CommitTransaction() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.InTransaction {
		return nil
	}

	if s.pgConn == nil {
		return fmt.Errorf("no PostgreSQL connection")
	}

	ctx := context.Background()
	_, err := s.pgConn.Exec(ctx, "COMMIT")
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.InTransaction = false
	return nil
}

func (s *Session) RollbackTransaction() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.InTransaction {
		return nil
	}

	if s.pgConn == nil {
		return fmt.Errorf("no PostgreSQL connection")
	}

	ctx := context.Background()
	_, err := s.pgConn.Exec(ctx, "ROLLBACK")
	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %w", err)
	}

	s.InTransaction = false
	return nil
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pgConn != nil {
		ctx := context.Background()
		if s.InTransaction {
			s.pgConn.Exec(ctx, "ROLLBACK")
		}
		err := s.pgConn.Close(ctx)
		s.pgConn = nil
		return err
	}

	return nil
}
