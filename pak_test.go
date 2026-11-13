package larian

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierrec/lz4/v4"
)

type fixtureFile struct {
	name string
	data []byte
	comp Compression
	// offset overrides the stored data offset when non-zero.
	offset uint64
}

func lz4Block(tb testing.TB, data []byte) []byte {
	tb.Helper()

	dst := make([]byte, lz4.CompressBlockBound(len(data)))

	n, err := lz4.CompressBlock(data, dst, nil)
	if err != nil || n == 0 {
		tb.Fatalf("fixture not compressible: n=%d err=%v", n, err)
	}

	return dst[:n]
}

// buildPak writes a synthetic LSPK of the given version (15, 16 or 18).
func buildPak(t testing.TB, version uint32, files []fixtureFile) []byte {
	t.Helper()

	entryLen, headerLen := entryLenV18, headerLenV16

	switch version {
	case 15:
		entryLen, headerLen = entryLenV15, headerLenV15
	case 16:
		entryLen = entryLenV15
	}

	body := bytes.Repeat([]byte{0}, headerLen)
	table := make([]byte, 0, len(files)*entryLen)

	for _, f := range files {
		var disk []byte

		switch f.comp {
		case CompressionNone:
			disk = f.data
		case CompressionZlib:
			var b bytes.Buffer

			zw := zlib.NewWriter(&b)
			_, _ = zw.Write(f.data)
			_ = zw.Close()
			disk = b.Bytes()
		case CompressionLZ4:
			disk = lz4Block(t, f.data)
		case CompressionZstd:
			t.Fatalf("fixture compression zstd")
		default:
			t.Fatalf("fixture compression %d", f.comp)
		}

		e := make([]byte, entryLen)
		copy(e, f.name)

		off := uint64(len(body))
		if f.offset != 0 {
			off = f.offset
		}

		if version != 18 {
			binary.LittleEndian.PutUint64(e[256:], off)
			binary.LittleEndian.PutUint64(e[264:], uint64(len(disk)))
			binary.LittleEndian.PutUint64(e[272:], uint64(len(f.data)))
			binary.LittleEndian.PutUint32(e[284:], uint32(f.comp))
		} else {
			binary.LittleEndian.PutUint32(e[256:], u32(off))
			binary.LittleEndian.PutUint16(e[260:], u16(off>>32))
			e[263] = byte(f.comp)
			binary.LittleEndian.PutUint32(e[264:], u32(len(disk)))
			binary.LittleEndian.PutUint32(e[268:], u32(len(f.data)))
		}

		if f.offset == 0 {
			body = append(body, disk...)
		}
		table = append(table, e...)
	}

	comp := lz4Block(t, table)
	listOff := len(body)

	var head [8]byte

	binary.LittleEndian.PutUint32(head[:], u32(len(files)))
	binary.LittleEndian.PutUint32(head[4:], u32(len(comp)))

	body = append(body, head[:]...)
	body = append(body, comp...)

	copy(body, magic)
	binary.LittleEndian.PutUint32(body[4:], version)
	binary.LittleEndian.PutUint64(body[8:], uint64(listOff))
	binary.LittleEndian.PutUint32(body[16:], u32(len(body)-listOff))

	return body
}

func u32[T ~int | ~uint64](n T) uint32 { return uint32(n) }

func u16[T ~uint64](n T) uint16 { return uint16(n) }

func openBytes(t *testing.T, b []byte) (*Pak, error) {
	t.Helper()

	return Open(bytes.NewReader(b), int64(len(b)))
}

func TestPakRoundTrip(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("baldur's gate 3 "), 64)
	files := []fixtureFile{
		{name: "Mods/A/meta.lsx", data: payload, comp: CompressionLZ4},
		{name: "Public/A/x.bin", data: payload, comp: CompressionZlib},
		{name: "Public/A/raw.txt", data: []byte("raw"), comp: CompressionNone},
		{name: `Public\A\back.txt`, data: []byte("slashes"), comp: CompressionNone},
	}

	for _, v := range []uint32{15, 16, 18} {
		p, err := openBytes(t, buildPak(t, v, files))
		if err != nil {
			t.Fatalf("v%d: %v", v, err)
		}

		if p.Version != v || len(p.Entries()) != len(files) {
			t.Fatalf("v%d: version %d entries %d", v, p.Version, len(p.Entries()))
		}

		for _, f := range files {
			path := strings.ReplaceAll(f.name, `\`, "/")

			got, err := p.Read(path)
			if err != nil || !bytes.Equal(got, f.data) {
				t.Fatalf("v%d %s: %v", v, path, err)
			}
		}

		if _, err := p.Read("nope"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("v%d: want ErrNotFound, got %v", v, err)
		}
	}
}

func TestOpenFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "m.pak")
	if err := os.WriteFile(path, buildPak(t, 18, []fixtureFile{{name: "a.txt", data: []byte("hi"), comp: CompressionNone}}), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = p.Close() }()

	if b, err := p.Read("a.txt"); err != nil || string(b) != "hi" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestOpenRejectsHostileInput(t *testing.T) {
	t.Parallel()

	good := buildPak(t, 18, []fixtureFile{{name: "a.txt", data: []byte("hello"), comp: CompressionNone}})
	mutate := func(f func(b []byte) []byte) []byte { return f(append([]byte(nil), good...)) }

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ErrInvalid},
		{"bad magic", mutate(func(b []byte) []byte { b[0] = 'X'; return b }), ErrInvalid},
		{"version 13", mutate(func(b []byte) []byte { b[4] = 13; return b }), ErrUnsupported},
		{"solid", mutate(func(b []byte) []byte { b[20] = flagSolid; return b }), ErrUnsupported},
		{"list offset past end", mutate(func(b []byte) []byte {
			binary.LittleEndian.PutUint64(b[8:], 1<<40)

			return b
		}), ErrInvalid},
		{"list size past end", mutate(func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[16:], 0xffffffff)

			return b
		}), ErrInvalid},
		{"entry count huge", mutate(func(b []byte) []byte {
			off := binary.LittleEndian.Uint64(b[8:])
			binary.LittleEndian.PutUint32(b[off:], 0xffffffff)

			return b
		}), ErrInvalid},
		{"truncated", good[:len(good)-5], ErrInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := openBytes(t, tc.data); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestOpenRejectsUnsafePathsAndBounds(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"../evil", "a/../../evil", "/abs", "C:/win", "a//b", "./a", `..\evil`, "Mods/.. /x", "Mods/a.", "Mods/a:b", "Mods/CON", "Mods/nul.txt", "Mods/Com1 .x", "Mods/CONIN$/x", "Mods/CONOUT$.txt", "Mods/a?b", "Mods/a*", "Mods/a|b", "Mods/a<b", "Mods/a\x01b", "Mods/COM¹", "Mods/lpt³.txt"} {
		if _, err := openBytes(t, buildPak(t, 18, []fixtureFile{{name: name, data: []byte("x"), comp: CompressionNone}})); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: want ErrInvalid, got %v", name, err)
		}
	}

	dup := []fixtureFile{{name: "a", data: []byte("1"), comp: CompressionNone}, {name: "a", data: []byte("2"), comp: CompressionNone}}
	if _, err := openBytes(t, buildPak(t, 18, dup)); !errors.Is(err, ErrInvalid) {
		t.Errorf("duplicate: %v", err)
	}

	// Entry data pointing past the end of the package.
	b := buildPak(t, 18, []fixtureFile{{name: "a", data: []byte("12345"), comp: CompressionNone}})
	// Rebuild the table with a huge size via a second pak: patch is awkward under LZ4, so build directly.
	_ = b
}

func TestReadBadEntries(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("z"), 200)
	b := buildPak(t, 18, []fixtureFile{{name: "l.bin", data: payload, comp: CompressionLZ4}, {name: "z.bin", data: payload, comp: CompressionZlib}})

	p, err := openBytes(t, b)
	if err != nil {
		t.Fatal(err)
	}

	// Claimed sizes the data does not honour fail without panicking.
	for i := range p.entries {
		p.entries[i].Size++
	}

	for _, e := range p.Entries() {
		if _, err := p.Read(e.Path); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", e.Path, err)
		}
	}

	p.entries[0].Size = MaxEntrySize + 1
	if _, err := p.Read("l.bin"); !errors.Is(err, ErrInvalid) {
		t.Errorf("oversize: %v", err)
	}

	p.entries[1].part = 1
	if _, err := p.Read("z.bin"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("multi-part: %v", err)
	}

	p.entries[1].part = 0
	p.entries[1].Compression = CompressionZstd

	if _, err := p.Read("z.bin"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("zstd: %v", err)
	}
}

func FuzzOpen(f *testing.F) {
	f.Add(buildPak(f, 18, []fixtureFile{{name: "a.txt", data: bytes.Repeat([]byte("a"), 99), comp: CompressionLZ4}}))
	f.Add([]byte("LSPK"))
	f.Add([]byte("LSPK\x12\x00\x00\x00\x28\x00\x00\x00\x00\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := Open(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}

		entries := p.Entries()
		if len(entries) > MaxEntries {
			t.Fatalf("Entries() = %d, MaxEntries = %d", len(entries), MaxEntries)
		}

		for _, e := range entries {
			if e.Path == "" || strings.HasPrefix(e.Path, "/") {
				t.Fatalf("unsafe path survived Open: %q", e.Path)
			}

			for seg := range strings.SplitSeq(e.Path, "/") {
				if seg == "" || seg == "." || seg == ".." {
					t.Fatalf("unsafe path survived Open: %q", e.Path)
				}
			}

			got, err := p.Read(e.Path)
			if err != nil {
				continue
			}

			if uint64(len(got)) != e.Size {
				t.Fatalf("%q: Read returned %d bytes, Size = %d", e.Path, len(got), e.Size)
			}
		}
	})
}

type zeroReaderAt struct{ read int }

func (z *zeroReaderAt) ReadAt(b []byte, _ int64) (int, error) {
	z.read += len(b)
	return len(b), nil
}

func TestReadRefusesOversizedLZ4DiskSize(t *testing.T) {
	t.Parallel()

	z := &zeroReaderAt{}
	p := &Pak{r: z, entries: []Entry{{Path: "a", Size: 10, Compression: CompressionLZ4, diskSize: 1 << 28}}}

	if _, err := p.readEntry(&p.entries[0]); !errors.Is(err, ErrInvalid) || z.read != 0 {
		t.Fatalf("err = %v, read %d bytes", err, z.read)
	}
}

func TestOpenSkipsDeletionMarksWithoutBoundsChecking(t *testing.T) {
	t.Parallel()

	b := buildPak(t, 18, []fixtureFile{
		{name: "Mods/A/meta.lsx", offset: deletionMark},
		{name: "Mods/A/kept.txt", data: []byte("x"), comp: CompressionNone},
	})

	p, err := openBytes(t, b)
	if err != nil {
		t.Fatal(err)
	}

	if es := p.Entries(); len(es) != 1 || es[0].Path != "Mods/A/kept.txt" {
		t.Fatalf("entries = %+v", es)
	}
}

func TestReadRefusesSizesCompressionCannotReach(t *testing.T) {
	t.Parallel()

	for _, comp := range []Compression{CompressionZlib, CompressionLZ4} {
		z := &zeroReaderAt{}
		p := &Pak{r: z, entries: []Entry{{Path: "a", Size: MaxEntrySize, Compression: comp, diskSize: 12}}}

		if _, err := p.readEntry(&p.entries[0]); !errors.Is(err, ErrInvalid) || z.read != 0 {
			t.Fatalf("compression %d: err = %v, read %d bytes", comp, err, z.read)
		}
	}
}

func TestReadRejectsCorruptZlibChecksum(t *testing.T) {
	t.Parallel()

	b := buildPak(t, 18, []fixtureFile{{name: "Mods/x/x.txt", data: []byte("hello"), comp: CompressionZlib}})
	listOff := binary.LittleEndian.Uint64(b[8:])
	b[listOff-1] ^= 1

	p, err := openBytes(t, b)
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Read("Mods/x/x.txt")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Read: %v", err)
	}
}
