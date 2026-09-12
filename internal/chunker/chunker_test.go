package chunker

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"
)

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func splitAll(t *testing.T, c *Chunker, data []byte) []Chunk {
	t.Helper()
	var chunks []Chunk
	err := c.Split(bytes.NewReader(data), func(ch Chunk) error {
		cp := make([]byte, len(ch.Data))
		copy(cp, ch.Data)
		chunks = append(chunks, Chunk{Index: ch.Index, Offset: ch.Offset, Data: cp})
		return nil
	})
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	return chunks
}

func randomBytes(n int, seed int64) []byte {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	r.Read(b)
	return b
}

func TestSplitReconstructsOriginalBytes(t *testing.T) {
	c := New(DefaultMinSize, DefaultAvgSize, DefaultMaxSize)
	data := randomBytes(5*1024*1024+37, 1)

	chunks := splitAll(t, c, data)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks for 5MB input, got %d", len(chunks))
	}

	var out bytes.Buffer
	for _, ch := range chunks {
		out.Write(ch.Data)
	}
	if !bytes.Equal(out.Bytes(), data) {
		t.Fatalf("reconstructed data does not match original")
	}
}

func TestChunkSizeBounds(t *testing.T) {
	minSize, avgSize, maxSize := 256, 1024, 4096
	c := New(minSize, avgSize, maxSize)
	data := randomBytes(2*1024*1024, 2)

	chunks := splitAll(t, c, data)
	for i, ch := range chunks {
		last := i == len(chunks)-1
		if len(ch.Data) > maxSize {
			t.Fatalf("chunk %d exceeds maxSize: %d > %d", i, len(ch.Data), maxSize)
		}
		if !last && len(ch.Data) < minSize {
			t.Fatalf("non-final chunk %d shorter than minSize: %d < %d", i, len(ch.Data), minSize)
		}
	}
}

func TestIdenticalContentProducesIdenticalChunks(t *testing.T) {
	c := New(256, 1024, 4096)

	shared := randomBytes(50*1024, 3)
	fileA := append(append([]byte("prefix-A--"), shared...), []byte("suffix-A")...)
	fileB := append(append([]byte("totally different prefix"), shared...), []byte("another suffix")...)

	chunksA := splitAll(t, c, fileA)
	chunksB := splitAll(t, c, fileB)

	hashesA := map[string]bool{}
	for _, ch := range chunksA {
		hashesA[hashOf(ch.Data)] = true
	}

	shared_found := 0
	for _, ch := range chunksB {
		if hashesA[hashOf(ch.Data)] {
			shared_found++
		}
	}

	if shared_found == 0 {
		t.Fatalf("expected at least one chunk shared between files with common content")
	}
}

func TestEmptyInput(t *testing.T) {
	c := New(DefaultMinSize, DefaultAvgSize, DefaultMaxSize)
	chunks := splitAll(t, c, nil)
	if len(chunks) != 0 {
		t.Fatalf("expected no chunks for empty input, got %d", len(chunks))
	}
}

func TestSmallInputSingleChunk(t *testing.T) {
	c := New(DefaultMinSize, DefaultAvgSize, DefaultMaxSize)
	data := []byte("hello, mosaic!")
	chunks := splitAll(t, c, data)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for small input, got %d", len(chunks))
	}
	if !bytes.Equal(chunks[0].Data, data) {
		t.Fatalf("chunk data mismatch")
	}
}

func TestNewPanicsOnInvalidBounds(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic for invalid bounds")
		}
	}()
	New(100, 50, 200)
}
