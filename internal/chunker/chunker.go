// Package chunker splits file content into variable-length, content-defined
// chunks ("tesserae" of the mosaic). Because chunk boundaries are derived
// from a rolling hash of the data itself rather than from fixed offsets,
// identical byte runs produce identical chunks even when they sit at
// different offsets in different files. That property is what lets the
// store deduplicate chunks across unrelated uploads.
package chunker

import (
	"bufio"
	"io"
)

// Default chunk size targets, in bytes. Average chunk size drives the
// dedup/overhead trade-off: smaller chunks find more duplicate content but
// add more bookkeeping rows per file.
const (
	DefaultMinSize = 2 * 1024
	DefaultAvgSize = 8 * 1024
	DefaultMaxSize = 64 * 1024
)

// Chunk is one content-defined slice of a file.
type Chunk struct {
	Index  int
	Offset int64
	Data   []byte
}

// Chunker implements a FastCDC-style normalized chunker using a gear hash.
type Chunker struct {
	minSize, avgSize, maxSize int
	maskS, maskL              uint64
}

// New builds a Chunker for the given size bounds. Panics if the bounds are
// not min < avg < max, since that would make the algorithm degenerate.
func New(minSize, avgSize, maxSize int) *Chunker {
	if !(minSize > 0 && minSize < avgSize && avgSize < maxSize) {
		panic("chunker: require 0 < minSize < avgSize < maxSize")
	}
	bits := 0
	for 1<<uint(bits+1) <= avgSize {
		bits++
	}
	return &Chunker{
		minSize: minSize,
		avgSize: avgSize,
		maxSize: maxSize,
		// Stricter mask before the average size discourages very short
		// chunks; looser mask after it discourages runaway long ones.
		// This is the "normalization" step from the FastCDC paper.
		maskS: maskWithBits(bits + 1),
		maskL: maskWithBits(bits - 1),
	}
}

func maskWithBits(bits int) uint64 {
	if bits <= 0 {
		return 0
	}
	if bits >= 64 {
		return ^uint64(0)
	}
	return (uint64(1) << uint(bits)) - 1
}

// Split reads r to completion, invoking onChunk once per content-defined
// chunk in order. onChunk must not retain the passed slice beyond the call.
func (c *Chunker) Split(r io.Reader, onChunk func(Chunk) error) error {
	br := bufio.NewReaderSize(r, 1<<20)
	buf := make([]byte, 0, c.maxSize)
	fill := make([]byte, c.maxSize)
	var offset int64
	index := 0

	for {
		for len(buf) < c.maxSize {
			n, err := br.Read(fill[:c.maxSize-len(buf)])
			if n > 0 {
				buf = append(buf, fill[:n]...)
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if n == 0 {
				break
			}
		}
		if len(buf) == 0 {
			return nil
		}

		cut := c.cutPoint(buf)
		chunk := Chunk{Index: index, Offset: offset, Data: buf[:cut]}
		if err := onChunk(chunk); err != nil {
			return err
		}

		offset += int64(cut)
		index++
		remainder := len(buf) - cut
		copy(buf, buf[cut:])
		buf = buf[:remainder]
	}
}

// cutPoint returns the length of the next chunk within data. data holds at
// most maxSize bytes; a length shorter than maxSize means we are at EOF.
func (c *Chunker) cutPoint(data []byte) int {
	n := len(data)
	if n <= c.minSize {
		return n
	}

	var hash uint64
	i := c.minSize

	small := c.avgSize
	if small > n {
		small = n
	}
	for ; i < small; i++ {
		hash = (hash << 1) + gearTable[data[i]]
		if hash&c.maskS == 0 {
			return i + 1
		}
	}

	large := c.maxSize
	if large > n {
		large = n
	}
	for ; i < large; i++ {
		hash = (hash << 1) + gearTable[data[i]]
		if hash&c.maskL == 0 {
			return i + 1
		}
	}

	return large
}
