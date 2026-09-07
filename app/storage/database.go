package storage

import (
	"context"
	"database/sql"

	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	// Serialize access to this SQLite file. All row sets must be closed before
	// a method issues another query; no connection is held during network I/O.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{DB: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
func (s *Store) Close() error { return s.DB.Close() }
