package pzip

import (
	"archive/zip"
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

type FileHeader = zip.FileHeader

const (
	fileHeaderSignature      = 0x04034b50
	directoryHeaderSignature = 0x02014b50
	directoryEndSignature    = 0x06054b50
	directory64LocSignature  = 0x07064b50
	directory64EndSignature  = 0x06064b50
	dataDescriptorSignature  = 0x08074b50 // de-facto standard; required by OS X Finder
	fileHeaderLen            = 30         // + filename + extra
	directoryHeaderLen       = 46         // + filename + extra + comment
	directoryEndLen          = 22         // + comment
	dataDescriptorLen        = 16         // four uint32: descriptor signature, crc32, compressed size, size
	dataDescriptor64Len      = 24         // two uint32: signature, crc32 | two uint64: compressed size, size
	directory64LocLen        = 20         //
	directory64EndLen        = 56         // + extra

	// Constants for the first byte in CreatorVersion.
	creatorFAT    = 0
	creatorUnix   = 3
	creatorNTFS   = 11
	creatorVFAT   = 14
	creatorMacOSX = 19

	// Version numbers.
	zipVersion20 = 20
	zipVersion45 = 45

	// Limits for non zip64 files.
	uint16max = (1 << 16) - 1
	uint32max = (1 << 32) - 1

	// Extra header IDs.
	//
	// IDs 0..31 are reserved for official use by PKWARE.
	// IDs above that range are defined by third-party vendors.
	// Since ZIP lacked high precision timestamps (nor an official specification
	// of the timezone used for the date fields), many competing extra fields
	// have been invented. Pervasive use effectively makes them "official".
	//
	// See http://mdfs.net/Docs/Comp/Archiving/Zip/ExtraField
	zip64ExtraID       = 0x0001 // Zip64 extended information
	ntfsExtraID        = 0x000a // NTFS
	unixExtraID        = 0x000d // UNIX
	extTimeExtraID     = 0x5455 // Extended timestamp
	infoZipUnixExtraID = 0x5855 // Info-ZIP Unix extension
)

type header struct {
	*FileHeader
	offset             uint64
	offset32           uint32
	compressedSize32   uint32
	uncompressedSize32 uint32
}

func (h *header) isZip64() bool {
	return h.CompressedSize64 >= uint32max || h.UncompressedSize64 >= uint32max
}

func (h *header) prepare() {
	//headerPrepareByAppend(h)
	headerPrepareByWriteBuf(h)
}

func headerPrepareByAppend(h *header) {
	h.offset32 = uint32(h.offset)
	h.compressedSize32 = h.CompressedSize
	h.uncompressedSize32 = h.UncompressedSize

	if h.isZip64() || h.offset >= uint32max {

		h.ReaderVersion = zipVersion45

		// 3x uint64
		zip64bufData := make([]byte, 0, 24)
		if h.isZip64() {
			zip64bufData = binary.LittleEndian.AppendUint64(zip64bufData, h.UncompressedSize64)
			zip64bufData = binary.LittleEndian.AppendUint64(zip64bufData, h.CompressedSize64)
			h.uncompressedSize32 = uint32max
			h.compressedSize32 = uint32max
		}

		if h.offset >= uint32max {
			zip64bufData = binary.LittleEndian.AppendUint64(zip64bufData, h.offset)
			h.offset32 = uint32max
		}

		// max len: 2x uint16 + 3x uint64
		zip64buf := make([]byte, 0, len(zip64bufData)+4)
		zip64buf = binary.LittleEndian.AppendUint16(zip64buf, zip64ExtraID)
		zip64buf = binary.LittleEndian.AppendUint16(zip64buf, uint16(len(zip64bufData)))
		zip64buf = append(zip64buf, zip64bufData...)
		h.Extra = append(h.Extra, zip64buf...)
	}
}

func headerPrepareByWriteBuf(h *header) {
	h.offset32 = uint32(h.offset)
	h.compressedSize32 = h.CompressedSize
	h.uncompressedSize32 = h.UncompressedSize

	if h.isZip64() || h.offset >= uint32max {

		h.ReaderVersion = zipVersion45

		var zip64buf [28]byte // 2x uint16 + 3x uint64
		eb := writeBuf(zip64buf[:])
		eb.uint16(zip64ExtraID)

		if h.isZip64() && h.offset >= uint32max {
			eb.uint16(24) // size = 3x uint64
			eb.uint64(h.UncompressedSize64)
			eb.uint64(h.CompressedSize64)
			eb.uint64(h.offset)
			h.uncompressedSize32 = uint32max
			h.compressedSize32 = uint32max
			h.offset32 = uint32max
		} else if h.isZip64() {
			eb.uint16(16) // size = 2x uint64
			eb.uint64(h.UncompressedSize64)
			eb.uint64(h.CompressedSize64)
			h.uncompressedSize32 = uint32max
			h.compressedSize32 = uint32max
		} else if h.offset >= uint32max {
			eb.uint16(8) // size = 1x uint64
			eb.uint64(h.offset)
			h.offset32 = uint32max
		}
		h.Extra = append(h.Extra, zip64buf[:len(zip64buf)-len(eb)]...)
	}
}

type Writer struct {
	cw  *countWriter
	dir []*header

	closed bool

	comment string
}

// NewWriter returns a new [Writer] writing a zip file to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{cw: &countWriter{w: bufio.NewWriter(w)}}
}

// Flush flushes any buffered data to the underlying writer.
// Calling Flush is not normally necessary; calling Close is sufficient.
func (w *Writer) Flush() error {
	return w.cw.w.Flush()
}

// SetComment sets the end-of-central-directory comment field.
// It can only be called before [Writer.Close].
func (w *Writer) SetComment(comment string) error {
	if len(comment) > uint16max {
		return errors.New("zip: Writer.Comment too long")
	}
	w.comment = comment
	return nil
}

// prepare performs the bookkeeping operations required at the start of
// CreateHeader and CreateRaw.
func (w *Writer) prepare(fh *FileHeader) error {
	if len(w.dir) > 0 && w.dir[len(w.dir)-1].FileHeader == fh {
		// See https://golang.org/issue/11144 confusion.
		return errors.New("archive/zip: invalid duplicate FileHeader")
	}
	return nil
}

func writeHeader(w io.Writer, h *header) error {
	if len(h.Name) > uint16max {
		return errors.New("zip: header name too long")
	}
	if len(h.Extra) > uint16max {
		return errors.New("zip: header extra too long")
	}
	// reference: https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT
	// 4.3.7  Local file header:
	h.prepare()

	var buf [fileHeaderLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(fileHeaderSignature))
	b.uint16(h.ReaderVersion)
	b.uint16(h.Flags)
	b.uint16(h.Method)
	b.uint16(h.ModifiedTime)
	b.uint16(h.ModifiedDate)
	b.uint32(h.CRC32)
	b.uint32(h.compressedSize32)
	b.uint32(h.uncompressedSize32)
	b.uint16(uint16(len(h.Name)))
	b.uint16(uint16(len(h.Extra)))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w, h.Name); err != nil {
		return err
	}
	_, err := w.Write(h.Extra)
	return err
}

// CreateRaw adds a file to the zip archive using the provided [FileHeader] and
// returns a [Writer] to which the file contents should be written. The file's
// contents must be written to the io.Writer before the next call to [Writer.Create],
// [Writer.CreateHeader], [Writer.CreateRaw], or [Writer.Close].
//
// In contrast to [Writer.CreateHeader], the bytes passed to Writer are not compressed.
func (w *Writer) CreateRaw(fh *FileHeader) (io.Writer, error) {
	if err := w.prepare(fh); err != nil {
		return nil, err
	}

	fh.CompressedSize = uint32(min(fh.CompressedSize64, uint32max))
	fh.UncompressedSize = uint32(min(fh.UncompressedSize64, uint32max))

	h := &header{
		FileHeader: fh,
		offset:     w.cw.count,
	}
	w.dir = append(w.dir, h)
	if err := writeHeader(w.cw, h); err != nil {
		return nil, err
	}

	if strings.HasSuffix(fh.Name, "/") {
		return dirWriter{}, nil
	}

	return w.cw, nil
}

func (w *Writer) Close() error {
	if w.closed {
		return errors.New("zip: writer closed twice")
	}
	w.closed = true

	// write central directory
	start := w.cw.count

	// reference: https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT
	// 4.3.12  Central directory structure:
	for _, h := range w.dir {
		var buf [directoryHeaderLen]byte
		b := writeBuf(buf[:])
		b.uint32(uint32(directoryHeaderSignature))
		b.uint16(h.CreatorVersion)
		b.uint16(h.ReaderVersion)
		b.uint16(h.Flags)
		b.uint16(h.Method)
		b.uint16(h.ModifiedTime)
		b.uint16(h.ModifiedDate)
		b.uint32(h.CRC32)
		b.uint32(h.compressedSize32)
		b.uint32(h.uncompressedSize32)
		b.uint16(uint16(len(h.Name)))
		b.uint16(uint16(len(h.Extra))) // Includes the Zip64 extra block if present
		b.uint16(uint16(len(h.Comment)))

		b = b[4:] // skip disk number start and internal file attr (2x uint16)
		b.uint32(h.ExternalAttrs)
		b.uint32(h.offset32)

		if _, err := w.cw.Write(buf[:]); err != nil {
			return err
		}
		if _, err := io.WriteString(w.cw, h.Name); err != nil {
			return err
		}
		if _, err := w.cw.Write(h.Extra); err != nil {
			return err
		}
		if _, err := io.WriteString(w.cw, h.Comment); err != nil {
			return err
		}

	}
	end := w.cw.count

	records := uint64(len(w.dir))
	size := uint64(end - start)
	offset := uint64(start)

	if records >= uint16max || size >= uint32max || offset >= uint32max {
		var buf [directory64EndLen + directory64LocLen]byte
		b := writeBuf(buf[:])

		// reference: https://pkware.cachefly.net/webdocs/casestudies/APPNOTE.TXT
		// 4.3.14  Zip64 end of central directory record
		b.uint32(directory64EndSignature)
		b.uint64(directory64EndLen - 12) // length minus signature (uint32) and length fields (uint64)
		b.uint16(zipVersion45)           // version made by
		b.uint16(zipVersion45)           // version needed to extract
		b.uint32(0)                      // number of this disk
		b.uint32(0)                      // number of the disk with the start of the central directory
		b.uint64(records)                // total number of entries in the central directory on this disk
		b.uint64(records)                // total number of entries in the central directory
		b.uint64(size)                   // size of the central directory
		b.uint64(offset)                 // offset of start of central directory with respect to the starting disk number

		// 4.3.15 Zip64 end of central directory locator
		b.uint32(directory64LocSignature)
		b.uint32(0)           // number of the disk with the start of the zip64 end of central directory
		b.uint64(uint64(end)) // relative offset of the zip64 end of central directory record
		b.uint32(1)           // total number of disks

		if _, err := w.cw.Write(buf[:]); err != nil {
			return err
		}

		// store max values in the regular end record to signal
		// that the zip64 values should be used instead
		records = uint16max
		size = uint32max
		offset = uint32max
	}

	// 4.3.16  End of central directory record:
	var buf [directoryEndLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(directoryEndSignature))
	b = b[4:]                        // skip over disk number and first disk number (2x uint16)
	b.uint16(uint16(records))        // number of entries this disk
	b.uint16(uint16(records))        // number of entries total
	b.uint32(uint32(size))           // size of directory
	b.uint32(uint32(offset))         // start of directory
	b.uint16(uint16(len(w.comment))) // byte size of EOCD comment
	if _, err := w.cw.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w.cw, w.comment); err != nil {
		return err
	}
	return w.Flush()
}

type dirWriter struct{}

func (dirWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return 0, errors.New("zip: write to directory")
}

type countWriter struct {
	w     *bufio.Writer
	count uint64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.count += uint64(n)
	return n, err
}

// detectUTF8 reports whether s is a valid UTF-8 string, and whether the string
// must be considered UTF-8 encoding (i.e., not compatible with CP-437, ASCII,
// or any other common encoding).
func detectUTF8(s string) (valid, require bool) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		// Officially, ZIP uses CP-437, but many readers use the system's
		// local character encoding. Most encoding are compatible with a large
		// subset of CP-437, which itself is ASCII-like.
		//
		// Forbid 0x7e and 0x5c since EUC-KR and Shift-JIS replace those
		// characters with localized currency and overline characters.
		if r < 0x20 || r > 0x7d || r == 0x5c {
			if !utf8.ValidRune(r) || (r == utf8.RuneError && size == 1) {
				return false, false
			}
			require = true
		}
	}
	return true, require
}

type writeBuf []byte

func (b *writeBuf) uint8(v uint8) {
	(*b)[0] = v
	*b = (*b)[1:]
}

func (b *writeBuf) uint16(v uint16) {
	binary.LittleEndian.PutUint16(*b, v)
	*b = (*b)[2:]
}

func (b *writeBuf) uint32(v uint32) {
	binary.LittleEndian.PutUint32(*b, v)
	*b = (*b)[4:]
}

func (b *writeBuf) uint64(v uint64) {
	binary.LittleEndian.PutUint64(*b, v)
	*b = (*b)[8:]
}

// isZip64 reports whether the file size exceeds the 32 bit limit
func isZip64(hdr *FileHeader) bool {
	return hdr.CompressedSize64 >= uint32max || hdr.UncompressedSize64 >= uint32max
}
