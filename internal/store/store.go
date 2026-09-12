// Package store persists deduplicated file content in a local SQLite
// database. Files are recorded as an ordered list of references into a
// content-addressed chunk table, so identical chunks uploaded as part of
// different files are stored on disk only once.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// FileRecord describes one uploaded file and its dedup stats.
type FileRecord struct {
	ID         int64
	Name       string
	Size       int64
	SHA256     string
	ChunkCount int
	NewBytes   int64 // bytes written to chunks that did not already exist
	CreatedAt  time.Time
}

// Store wraps a SQLite connection with the schema this app needs.
type Store struct {
	db *sql.DB
}

// Open creates (if needed) and opens the SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// SQLite handles one writer at a time; a single connection avoids
	// SQLITE_BUSY errors from concurrent writers within this process.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS files (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	name         TEXT NOT NULL,
	size         INTEGER NOT NULL,
	sha256       TEXT NOT NULL,
	chunk_count  INTEGER NOT NULL,
	new_bytes    INTEGER NOT NULL,
	created_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS chunks (
	hash      TEXT PRIMARY KEY,
	data      BLOB NOT NULL,
	size      INTEGER NOT NULL,
	ref_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS file_chunks (
	file_id    INTEGER NOT NULL REFERENCES files(id) ON DELETE CASCADE,
	idx        INTEGER NOT NULL,
	chunk_hash TEXT NOT NULL REFERENCES chunks(hash),
	PRIMARY KEY (file_id, idx)
);

CREATE INDEX IF NOT EXISTS idx_file_chunks_hash ON file_chunks(chunk_hash);
`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
