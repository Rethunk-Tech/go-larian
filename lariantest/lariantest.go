// Package lariantest builds in-memory LSPK packages for tests of code that reads them.
package lariantest

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/pierrec/lz4/v4"

	"github.com/Rethunk-Tech/go-larian"
)

// BuildPak returns an LSPK package of the given version (15, 16 or 18) holding
// files, keyed by slash path, each stored with compression c (none, zlib or
// LZ4). Files are written in sorted path order. It fails the test for any other
// version or compression.
func BuildPak(tb testing.TB, version int, files map[string][]byte, c larian.Compression) []byte {
	tb.Helper()

	headerLen, entryLen := 40, 296
	switch version {
	case 15:
		headerLen = 38
	case 18:
		entryLen = 272
	case 16:
	default:
		tb.Fatalf("lariantest: unsupported version %d", version)
	}

	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}

	slices.Sort(names)

	body := bytes.Repeat([]byte{0}, headerLen)
	table := make([]byte, 0, len(names)*entryLen)

	for _, n := range names {
		data := files[n]
		disk := encode(tb, data, c)
		e := make([]byte, entryLen)
		copy(e, n)

		off := uint64(len(body))
		if version == 18 {
			binary.LittleEndian.PutUint32(e[256:], uint32(off))     //nolint:gosec // low 32 bits by format
			binary.LittleEndian.PutUint16(e[260:], uint16(off>>32)) //nolint:gosec // bits 32..47 by format
			e[263] = byte(c)
			binary.LittleEndian.PutUint32(e[264:], uint32(len(disk))) //nolint:gosec // test sizes are small
			binary.LittleEndian.PutUint32(e[268:], uint32(len(data))) //nolint:gosec // test sizes are small
		} else {
			binary.LittleEndian.PutUint64(e[256:], off)
			binary.LittleEndian.PutUint64(e[264:], uint64(len(disk)))
			binary.LittleEndian.PutUint64(e[272:], uint64(len(data)))
			binary.LittleEndian.PutUint32(e[284:], uint32(c))
		}

		body = append(body, disk...)
		table = append(table, e...)
	}

	comp := lz4Block(tb, table)
	listOff := len(body)

	var head [8]byte

	binary.LittleEndian.PutUint32(head[:], uint32(len(names))) //nolint:gosec // test sizes are small
	binary.LittleEndian.PutUint32(head[4:], uint32(len(comp))) //nolint:gosec // test sizes are small
	body = append(body, head[:]...)
	body = append(body, comp...)

	copy(body, "LSPK")
	binary.LittleEndian.PutUint32(body[4:], uint32(version)) //nolint:gosec // 15, 16 or 18
	binary.LittleEndian.PutUint64(body[8:], uint64(listOff))
	binary.LittleEndian.PutUint32(body[16:], uint32(len(body)-listOff)) //nolint:gosec // test sizes are small

	return body
}

func encode(tb testing.TB, data []byte, c larian.Compression) []byte {
	tb.Helper()

	switch c {
	case larian.CompressionNone:
		return data
	case larian.CompressionZlib:
		var b bytes.Buffer

		zw := zlib.NewWriter(&b)
		_, _ = zw.Write(data)
		_ = zw.Close()

		return b.Bytes()
	case larian.CompressionLZ4:
		return lz4Block(tb, data)
	case larian.CompressionZstd:
		tb.Fatalf("lariantest: zstd is not supported")

		return nil
	default:
		tb.Fatalf("lariantest: unsupported compression %d", c)

		return nil
	}
}

func lz4Block(tb testing.TB, data []byte) []byte {
	tb.Helper()

	dst := make([]byte, lz4.CompressBlockBound(len(data)))

	n, err := lz4.CompressBlock(data, dst, nil)
	if err != nil || n == 0 {
		tb.Fatalf("lariantest: %d bytes not LZ4-compressible (n=%d, err=%v)", len(data), n, err)
	}

	return dst[:n]
}
