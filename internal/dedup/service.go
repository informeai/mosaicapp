// Package dedup ties the content-defined chunker to the SQLite store,
// exposing the two operations the UI needs: ingest a file's bytes, and
// reconstruct a previously ingested file back into bytes.
package dedup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/informeai/mosaicapp/internal/chunker"
	"github.com/informeai/mosaicapp/internal/store"
)

// FileInfo is the dedup-aware view of a stored file shown in the UI.
type FileInfo struct {
	ID           int64
	Name         string
	Size         int64
	SHA256       string
	ChunkCount   int
	NewBytes     int64 // bytes actually written to new chunks at upload time
	SavedBytes   int64 // Size - NewBytes: bytes not re-stored thanks to dedup
	SavedPercent float64
	CreatedAt    time.Time
}

// Service is the application-level dedup engine.
type Service struct {
	store   *store.Store
	chunker *chunker.Chunker
}

// New builds a Service backed by st, chunking content with the given size
// bounds (bytes). Pass zero for all three to use the package defaults.
func New(st *store.Store, minSize, avgSize, maxSize int) *Service {
	if minSize == 0 && avgSize == 0 && maxSize == 0 {
		minSize, avgSize, maxSize = chunker.DefaultMinSize, chunker.DefaultAvgSize, chunker.DefaultMaxSize
	}
	return &Service{
		store:   st,
		chunker: chunker.New(minSize, avgSize, maxSize),
	}
}

// Ingest splits data into content-defined chunks, stores any chunk whose
// hash is not already present, and records a new file entry referencing
// the full ordered chunk sequence.
func (s *Service) Ingest(name string, data []byte) (FileInfo, error) {
	whole := sha256.Sum256(data)

	var refs []store.ChunkRef
	err := s.chunker.Split(bytes.NewReader(data), func(c chunker.Chunk) error {
		sum := sha256.Sum256(c.Data)
		cp := make([]byte, len(c.Data))
		copy(cp, c.Data)
		refs = append(refs, store.ChunkRef{
			Index: c.Index,
			Hash:  hex.EncodeToString(sum[:]),
			Data:  cp,
		})
		return nil
	})
	if err != nil {
		return FileInfo{}, err
	}

	rec, err := s.store.IngestFile(name, int64(len(data)), hex.EncodeToString(whole[:]), refs)
	if err != nil {
		return FileInfo{}, err
	}
	return toFileInfo(*rec), nil
}

// List returns every stored file, most recently uploaded first.
func (s *Service) List() ([]FileInfo, error) {
	recs, err := s.store.ListFiles()
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, len(recs))
	for i, r := range recs {
		out[i] = toFileInfo(r)
	}
	return out, nil
}

// Reconstruct reassembles a stored file's original bytes from its chunks.
func (s *Service) Reconstruct(fileID int64) (name string, data []byte, err error) {
	return s.store.ReconstructFile(fileID)
}

// Close releases the underlying database connection.
func (s *Service) Close() error {
	return s.store.Close()
}

func toFileInfo(r store.FileRecord) FileInfo {
	saved := r.Size - r.NewBytes
	var pct float64
	if r.Size > 0 {
		pct = float64(saved) / float64(r.Size) * 100
	}
	return FileInfo{
		ID:           r.ID,
		Name:         r.Name,
		Size:         r.Size,
		SHA256:       r.SHA256,
		ChunkCount:   r.ChunkCount,
		NewBytes:     r.NewBytes,
		SavedBytes:   saved,
		SavedPercent: pct,
		CreatedAt:    r.CreatedAt,
	}
}
