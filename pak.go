package larian

import (
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/pierrec/lz4/v4"
)

// Limits on what a package may claim. They bound allocation, not the format.
const (
	// MaxEntries is the most files a package may list.
	MaxEntries = 1 << 18
	// MaxEntrySize is the largest uncompressed file [Pak.Read] will allocate.
	MaxEntrySize = 1 << 30
)

const (
	magic        = "LSPK"
	nameLen      = 256
	entryLenV15  = 296
	entryLenV18  = 272
	headerLenV15 = 38
	headerLenV16 = 40
	// flagSolid is the package header's flag for one LZ4 frame holding every file.
	flagSolid = 0x04
	// deletionMark marks (in the low 48 offset bits) a patch entry that deletes a file.
	deletionMark = 0xbeefdeadbeef

	// The most a compressed stream can expand by; a declared size past it is refused before the output is allocated.
	maxDeflateRatio = 1032
	maxLZ4Ratio     = 255
)

// Errors returned by this package; test with [errors.Is].
var (
	// ErrInvalid means the data is not a well-formed package.
	ErrInvalid = errors.New("larian: invalid package")
	// ErrUnsupported means the package uses a version or feature this reader lacks.
	ErrUnsupported = errors.New("larian: unsupported package feature")
	// ErrNotFound means the package has no such file.
	ErrNotFound = errors.New("larian: file not found in package")
)

// Compression is an entry's storage method.
type Compression uint8

// Compression methods, as the low nibble of the entry flags.
const (
	CompressionNone Compression = 0
	CompressionZlib Compression = 1
	CompressionLZ4  Compression = 2
	CompressionZstd Compression = 3
)

// Entry describes one file in a package.
type Entry struct {
	// Path is slash-separated, relative, and free of "." and ".." segments.
	Path string
	// Size is the uncompressed size in bytes.
	Size        uint64
	Compression Compression

	offset   uint64
	diskSize uint64
	part     uint8
}

// Pak is an open LSPK package.
type Pak struct {
	// Version is the package format version (15, 16 or 18).
	Version uint32

	r       io.ReaderAt
	closer  io.Closer
	size    uint64
	entries []Entry
	index   map[string]int
}

// OpenFile opens the package at path. Close it when done.
func OpenFile(path string) (*Pak, error) {
	// O_NONBLOCK keeps the open of a FIFO from waiting for a writer; the regular-file check below then refuses it.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) //nolint:gosec // opening the caller-named package is the function's purpose
	if err != nil {
		return nil, fmt.Errorf("larian: %w", err)
	}

	st, err := f.Stat()
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("larian: %w", err)
	}

	if !st.Mode().IsRegular() {
		_ = f.Close()

		return nil, fmt.Errorf("%w: %s is not a regular file", ErrInvalid, path)
	}

	p, err := Open(f, st.Size())
	if err != nil {
		_ = f.Close()

		return nil, err
	}

	p.closer = f

	return p, nil
}

// Close releases the file behind [OpenFile]; it is a no-op for [Open].
func (p *Pak) Close() error {
	if p.closer == nil {
		return nil
	}

	return p.closer.Close()
}

// Open reads the header and file list of the size-byte package behind r.
func Open(r io.ReaderAt, size int64) (*Pak, error) {
	if size < 0 {
		return nil, fmt.Errorf("%w: negative size", ErrInvalid)
	}

	var hdr [headerLenV16]byte

	n, err := r.ReadAt(hdr[:], 0)
	if n < 8 || (err != nil && !errors.Is(err, io.EOF)) {
		return nil, fmt.Errorf("%w: short header", ErrInvalid)
	}

	if string(hdr[:4]) != magic {
		return nil, fmt.Errorf("%w: bad signature", ErrInvalid)
	}

	version := binary.LittleEndian.Uint32(hdr[4:])

	var entryLen, headerLen int

	switch version {
	case 15:
		entryLen, headerLen = entryLenV15, headerLenV15
	case 16:
		entryLen, headerLen = entryLenV15, headerLenV16
	case 18:
		entryLen, headerLen = entryLenV18, headerLenV16
	default:
		return nil, fmt.Errorf("%w: version %d", ErrUnsupported, version)
	}

	if n < headerLen {
		return nil, fmt.Errorf("%w: short header", ErrInvalid)
	}

	if hdr[20]&flagSolid != 0 {
		return nil, fmt.Errorf("%w: solid package", ErrUnsupported)
	}

	listOff := binary.LittleEndian.Uint64(hdr[8:])
	listSize := binary.LittleEndian.Uint32(hdr[16:])

	if listOff < uint64(headerLen) || listOff > uint64(size) || uint64(listSize) > uint64(size)-listOff || listSize < 8 {
		return nil, fmt.Errorf("%w: file list out of bounds", ErrInvalid)
	}

	p := &Pak{Version: version, r: r, size: uint64(size)}

	table := make([]byte, listSize)
	if _, err := r.ReadAt(table, int64(listOff)); err != nil { //nolint:gosec // listOff <= size, an int64
		return nil, fmt.Errorf("%w: file list: %w", ErrInvalid, err)
	}

	if err := p.parseTable(table, entryLen); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *Pak) parseTable(table []byte, entryLen int) error {
	count := binary.LittleEndian.Uint32(table)
	compSize := binary.LittleEndian.Uint32(table[4:])

	if count > MaxEntries {
		return fmt.Errorf("%w: %d entries", ErrInvalid, count)
	}

	if uint64(compSize) > uint64(len(table))-8 {
		return fmt.Errorf("%w: file list size", ErrInvalid)
	}

	raw := make([]byte, int(count)*entryLen)

	got, err := lz4.UncompressBlock(table[8:8+compSize], raw)
	if err != nil || got != len(raw) {
		return fmt.Errorf("%w: file list does not decompress to %d entries", ErrInvalid, count)
	}

	p.entries = make([]Entry, 0, count)
	p.index = make(map[string]int, count)

	for i := range int(count) {
		e, err := p.parseEntry(raw[i*entryLen:(i+1)*entryLen], entryLen)
		if err != nil {
			return err
		}

		if e.deletes() {
			continue
		}

		if _, dup := p.index[e.Path]; dup {
			return fmt.Errorf("%w: duplicate path %q", ErrInvalid, e.Path)
		}

		p.index[e.Path] = len(p.entries)
		p.entries = append(p.entries, e)
	}

	return nil
}

func (p *Pak) parseEntry(b []byte, entryLen int) (Entry, error) {
	name, _, _ := strings.Cut(string(b[:nameLen]), "\x00")

	clean, ok := cleanPath(name)
	if !ok {
		return Entry{}, fmt.Errorf("%w: unsafe path %q", ErrInvalid, name)
	}

	e := Entry{Path: clean}

	var flags uint32

	if entryLen == entryLenV15 {
		e.offset = binary.LittleEndian.Uint64(b[256:])
		e.diskSize = binary.LittleEndian.Uint64(b[264:])
		e.Size = binary.LittleEndian.Uint64(b[272:])
		e.part = uint8(min(binary.LittleEndian.Uint32(b[280:]), 255))
		flags = binary.LittleEndian.Uint32(b[284:])
	} else {
		e.offset = uint64(binary.LittleEndian.Uint32(b[256:])) | uint64(binary.LittleEndian.Uint16(b[260:]))<<32
		e.part = b[262]
		flags = uint32(b[263])
		e.diskSize = uint64(binary.LittleEndian.Uint32(b[264:]))
		e.Size = uint64(binary.LittleEndian.Uint32(b[268:]))
	}

	e.Compression = Compression(flags & 0x0f)

	if e.Compression == CompressionNone {
		e.Size = e.diskSize
	}

	if e.part == 0 && !e.deletes() && (e.offset > p.size || e.diskSize > p.size-e.offset) {
		return Entry{}, fmt.Errorf("%w: %q data out of bounds", ErrInvalid, clean)
	}

	return e, nil
}

// deletes reports a patch entry that removes a file; its offset is a marker, not a position.
func (e *Entry) deletes() bool { return e.offset&(1<<48-1) == deletionMark }

// cleanPath normalises a stored name to a safe relative slash path.
func cleanPath(name string) (string, bool) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") {
		return "", false
	}

	for seg := range strings.SplitSeq(name, "/") {
		if seg == "" || seg == "." || seg == ".." || !windowsSafeSegment(seg) {
			return "", false
		}
	}

	return name, true
}

// windowsSafeSegment rejects what Windows resolves to something other than the stored name: a trailing dot or space
// is stripped (so ".. " becomes ".."), ":" opens an alternate data stream or a drive, and a device name ("CON",
// "NUL.txt", "COM¹") names the device; the reserved characters and control characters cannot appear in a Windows name.
func windowsSafeSegment(seg string) bool {
	if strings.ContainsAny(seg, ":<>\"|?*") || strings.IndexFunc(seg, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 || strings.HasSuffix(seg, ".") || strings.HasSuffix(seg, " ") {
		return false
	}

	stem, _, _ := strings.Cut(seg, ".")
	switch strings.ToUpper(strings.TrimRight(stem, " ")) {
	case "CON", "CONIN$", "CONOUT$", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		"COM¹", "COM²", "COM³", "LPT¹", "LPT²", "LPT³":
		return false
	}

	return true
}

// Entries lists the package's files in stored order. The slice is a copy.
func (p *Pak) Entries() []Entry {
	return append([]Entry(nil), p.entries...)
}

// Read returns the uncompressed contents of the file at path (slash-separated,
// matched exactly). It returns [ErrNotFound] if absent.
func (p *Pak) Read(path string) ([]byte, error) {
	i, ok := p.index[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, path)
	}

	return p.readEntry(&p.entries[i])
}

func (p *Pak) readEntry(e *Entry) ([]byte, error) {
	if e.part != 0 {
		return nil, fmt.Errorf("%w: %q is in archive part %d", ErrUnsupported, e.Path, e.part)
	}

	if e.Size > MaxEntrySize {
		return nil, fmt.Errorf("%w: %q is %d bytes", ErrInvalid, e.Path, e.Size)
	}

	disk := io.NewSectionReader(p.r, int64(e.offset), int64(e.diskSize)) //nolint:gosec // bounds-checked against the package size in parseEntry

	switch e.Compression {
	case CompressionNone:
		return readExactly(disk, e)
	case CompressionZlib:
		if e.Size > e.diskSize*maxDeflateRatio {
			return nil, fmt.Errorf("%w: %q claims more than deflate can expand to", ErrInvalid, e.Path)
		}

		zr, err := zlib.NewReader(disk)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %w", ErrInvalid, e.Path, err)
		}

		defer func() { _ = zr.Close() }()

		return readExactly(zr, e)
	case CompressionLZ4:
		if e.diskSize > e.Size+e.Size/255+16 {
			return nil, fmt.Errorf("%w: %q is larger on disk than LZ4 allows", ErrInvalid, e.Path)
		}

		if e.Size > e.diskSize*maxLZ4Ratio {
			return nil, fmt.Errorf("%w: %q claims more than LZ4 can expand to", ErrInvalid, e.Path)
		}

		src, err := io.ReadAll(disk)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %w", ErrInvalid, e.Path, err)
		}

		dst := make([]byte, e.Size)

		n, err := lz4.UncompressBlock(src, dst)
		if err != nil || uint64(n) != e.Size { //nolint:gosec // n >= 0
			return nil, fmt.Errorf("%w: %q does not decompress to %d bytes", ErrInvalid, e.Path, e.Size)
		}

		return dst, nil
	case CompressionZstd:
		return nil, fmt.Errorf("%w: %q uses zstd", ErrUnsupported, e.Path)
	default:
		return nil, fmt.Errorf("%w: compression method %d", ErrUnsupported, e.Compression)
	}
}

// readExactly reads e.Size bytes and fails if the stream is shorter or longer.
func readExactly(r io.Reader, e *Entry) ([]byte, error) {
	buf := make([]byte, e.Size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalid, e.Path, err)
	}

	var extra [1]byte
	n, err := r.Read(extra[:])
	if n != 0 {
		return nil, fmt.Errorf("%w: %q is longer than its recorded size", ErrInvalid, e.Path)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalid, e.Path, err)
	}

	return buf, nil
}
