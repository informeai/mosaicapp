package dedup

import (
	"bytes"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/informeai/mosaicapp/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "mosaic.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, 256, 1024, 4096)
}

func TestIngestAndReconstructRoundTrip(t *testing.T) {
	svc := newTestService(t)

	r := rand.New(rand.NewSource(7))
	data := make([]byte, 300*1024)
	r.Read(data)

	info, err := svc.Ingest("dataset.bin", data)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if info.Size != int64(len(data)) {
		t.Fatalf("expected size %d, got %d", len(data), info.Size)
	}
	if info.NewBytes != info.Size {
		t.Fatalf("first ingest of unique data should report all bytes as new: %d vs %d", info.NewBytes, info.Size)
	}

	name, out, err := svc.Reconstruct(info.ID)
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if name != "dataset.bin" {
		t.Fatalf("unexpected name %q", name)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("reconstructed data mismatch")
	}
}

func TestSecondUploadOfSameContentSavesBytes(t *testing.T) {
	svc := newTestService(t)

	r := rand.New(rand.NewSource(11))
	data := make([]byte, 150*1024)
	r.Read(data)

	if _, err := svc.Ingest("first.bin", data); err != nil {
		t.Fatalf("Ingest first: %v", err)
	}
	second, err := svc.Ingest("copy.bin", data)
	if err != nil {
		t.Fatalf("Ingest second: %v", err)
	}

	if second.NewBytes != 0 {
		t.Fatalf("expected no new bytes for duplicate content, got %d", second.NewBytes)
	}
	if second.SavedBytes != second.Size {
		t.Fatalf("expected all bytes to be reported as saved, got %d of %d", second.SavedBytes, second.Size)
	}
	if second.SavedPercent != 100 {
		t.Fatalf("expected 100%% saved, got %.2f", second.SavedPercent)
	}
}

func TestListOrdersMostRecentFirst(t *testing.T) {
	svc := newTestService(t)

	if _, err := svc.Ingest("one.bin", []byte("one")); err != nil {
		t.Fatalf("Ingest one: %v", err)
	}
	if _, err := svc.Ingest("two.bin", []byte("two")); err != nil {
		t.Fatalf("Ingest two: %v", err)
	}

	list, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 files, got %d", len(list))
	}
	if list[0].Name != "two.bin" || list[1].Name != "one.bin" {
		t.Fatalf("expected most-recent-first order, got %s, %s", list[0].Name, list[1].Name)
	}
}
