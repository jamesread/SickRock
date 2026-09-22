package mysqlstore

import (
	"context"
	"database/sql"
)

// MySQL implements store.Store on a shared *sql.DB (MySQL dialect).
type MySQL struct {
	db *sql.DB
}

// NewMySQL wraps an existing MySQL connection. The caller applies migrations.
func NewMySQL(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

func (s *MySQL) DB() *sql.DB {
	return s.db
}

func (s *MySQL) Close() error {
	return s.db.Close()
}

func (s *MySQL) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
