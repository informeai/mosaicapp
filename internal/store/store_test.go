package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/informeai/mosaicapp/internal/chunker"
)

func randomBytes(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	r.Read(b)
	return b
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "mosaic.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func ingest(t *testing.T, s *Store, c *chunker.Chunker, name string, data []byte) *FileRecord {
	t.Helper()

	var refs []ChunkRef
	err := c.Split(bytes.NewReader(data), func(ch chunker.Chunk) error {
		sum := sha256.Sum256(ch.Data)
		cp := make([]byte, len(ch.Data))
		copy(cp, ch.Data)
		refs = append(refs, ChunkRef{Index: ch.Index, Hash: hex.EncodeToString(sum[:]), Data: cp})
		return nil
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}

	whole := sha256.Sum256(data)
	rec, err := s.IngestFile(name, int64(len(data)), hex.EncodeToString(whole[:]), refs)
	if err != nil {
		t.Fatalf("IngestFile: %v", err)
	}
	return rec
}

func TestIngestListReconstruct(t *testing.T) {
	s := openTestStore(t)
	c := chunker.New(256, 1024, 4096)

	data := bytes.Repeat([]byte("mosaic-tile-"), 2000)
	rec := ingest(t, s, c, "photo.bin", data)

	files, err := s.ListFiles()
	if err != nil {
		t.Fatalf("ListFiles: %v", err)
	}
	if len(files) != 1 || files[0].ID != rec.ID {
		t.Fatalf("expected one file matching ingest result, got %+v", files)
	}

	name, out, err := s.ReconstructFile(rec.ID)
	if err != nil {
		t.Fatalf("ReconstructFile: %v", err)
	}
	if name != "photo.bin" {
		t.Fatalf("expected name photo.bin, got %s", name)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("reconstructed bytes do not match original")
	}
}

func TestDuplicateContentIsDeduplicated(t *testing.T) {
	s := openTestStore(t)
	c := chunker.New(256, 1024, 4096)

	// Random (non-periodic) data so chunks are, with overwhelming
	// probability, unique within a single file; that isolates the
	// cross-file dedup behavior this test targets.
	data := randomBytes(200*1024, 42)
	first := ingest(t, s, c, "a.bin", data)

	countChunkRows := func() int {
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&n); err != nil {
			t.Fatalf("count chunks: %v", err)
		}
		return n
	}
	rowsAfterFirst := countChunkRows()
	if rowsAfterFirst == 0 {
		t.Fatalf("expected at least one chunk row after first ingest")
	}

	second := ingest(t, s, c, "b.bin", data)

	if second.NewBytes != 0 {
		t.Fatalf("expected zero new bytes for duplicate upload, got %d", second.NewBytes)
	}
	if first.ChunkCount != second.ChunkCount {
		t.Fatalf("expected same chunk count for identical content: %d vs %d", first.ChunkCount, second.ChunkCount)
	}
	if rowsAfterSecond := countChunkRows(); rowsAfterSecond != rowsAfterFirst {
		t.Fatalf("expected chunk row count unchanged by duplicate upload: %d vs %d", rowsAfterFirst, rowsAfterSecond)
	}

	_, outB, err := s.ReconstructFile(second.ID)
	if err != nil {
		t.Fatalf("ReconstructFile: %v", err)
	}
	if !bytes.Equal(outB, data) {
		t.Fatalf("reconstructed duplicate file does not match original")
	}
}

func TestReconstructMissingFile(t *testing.T) {
	s := openTestStore(t)
	if _, _, err := s.ReconstructFile(999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
