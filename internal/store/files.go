package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when a referenced file id does not exist.
var ErrNotFound = errors.New("store: file not found")

const timeLayout = time.RFC3339Nano

// ChunkRef is one content-defined chunk to be persisted as part of a file,
// in the order it occurs within that file.
type ChunkRef struct {
	Index int
	Hash  string
	Data  []byte
}

// IngestFile records a new file as an ordered sequence of chunks. Chunks
// whose hash already exists in the store are only reference-counted, not
// rewritten, which is where the deduplication savings come from.
func (s *Store) IngestFile(name string, size int64, sha256Hex string, chunks []ChunkRef) (*FileRecord, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	createdAt := time.Now().UTC()
	res, err := tx.Exec(
		`INSERT INTO files (name, size, sha256, chunk_count, new_bytes, created_at) VALUES (?, ?, ?, ?, 0, ?)`,
		name, size, sha256Hex, len(chunks), createdAt.Format(timeLayout),
	)
	if err != nil {
		return nil, fmt.Errorf("insert file: %w", err)
	}
	fileID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read new file id: %w", err)
	}

	var newBytes int64
	for _, c := range chunks {
		var dummy int
		err := tx.QueryRow(`SELECT 1 FROM chunks WHERE hash = ?`, c.Hash).Scan(&dummy)
		exists := true
		switch {
		case errors.Is(err, sql.ErrNoRows):
			exists = false
		case err != nil:
			return nil, fmt.Errorf("lookup chunk %s: %w", c.Hash, err)
		}

		if exists {
			if _, err := tx.Exec(`UPDATE chunks SET ref_count = ref_count + 1 WHERE hash = ?`, c.Hash); err != nil {
				return nil, fmt.Errorf("update chunk ref_count: %w", err)
			}
		} else {
			if _, err := tx.Exec(
				`INSERT INTO chunks (hash, data, size, ref_count) VALUES (?, ?, ?, 1)`,
				c.Hash, c.Data, len(c.Data),
			); err != nil {
				return nil, fmt.Errorf("insert chunk: %w", err)
			}
			newBytes += int64(len(c.Data))
		}

		if _, err := tx.Exec(
			`INSERT INTO file_chunks (file_id, idx, chunk_hash) VALUES (?, ?, ?)`,
			fileID, c.Index, c.Hash,
		); err != nil {
			return nil, fmt.Errorf("insert file_chunks: %w", err)
		}
	}

	if _, err := tx.Exec(`UPDATE files SET new_bytes = ? WHERE id = ?`, newBytes, fileID); err != nil {
		return nil, fmt.Errorf("update file new_bytes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &FileRecord{
		ID:         fileID,
		Name:       name,
		Size:       size,
		SHA256:     sha256Hex,
		ChunkCount: len(chunks),
		NewBytes:   newBytes,
		CreatedAt:  createdAt,
	}, nil
}

// ListFiles returns every stored file, most recently uploaded first.
func (s *Store) ListFiles() ([]FileRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, name, size, sha256, chunk_count, new_bytes, created_at FROM files ORDER BY id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	var out []FileRecord
	for rows.Next() {
		var f FileRecord
		var createdAt string
		if err := rows.Scan(&f.ID, &f.Name, &f.Size, &f.SHA256, &f.ChunkCount, &f.NewBytes, &createdAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		t, err := time.Parse(timeLayout, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		f.CreatedAt = t
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// ReconstructFile reassembles a previously ingested file's original bytes
// by concatenating its chunks in their stored order.
func (s *Store) ReconstructFile(fileID int64) (name string, data []byte, err error) {
	var size int64
	err = s.db.QueryRow(`SELECT name, size FROM files WHERE id = ?`, fileID).Scan(&name, &size)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("lookup file: %w", err)
	}

	rows, err := s.db.Query(
		`SELECT c.data FROM file_chunks fc
		 JOIN chunks c ON c.hash = fc.chunk_hash
		 WHERE fc.file_id = ?
		 ORDER BY fc.idx ASC`,
		fileID,
	)
	if err != nil {
		return "", nil, fmt.Errorf("query chunks: %w", err)
	}
	defer rows.Close()

	buf := make([]byte, 0, size)
	for rows.Next() {
		var chunk []byte
		if err := rows.Scan(&chunk); err != nil {
			return "", nil, fmt.Errorf("scan chunk: %w", err)
		}
		buf = append(buf, chunk...)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}

	return name, buf, nil
}
