package pzip

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/klauspost/compress/flate"
)

const (
	// defaultBufSize 2MB
	defaultBufSize = 1 << 21

	overflowPrefix = "pzip-overflow"
)

var DefaultObjectPool = NewObjectPool()

type Object struct {
	Root string
	Path string
	Info os.FileInfo

	compressedData  *bytes.Buffer
	compressor      *flate.Writer
	header          *FileHeader
	overflow        *os.File
	written         uint64
	compressMinSize uint64
	link            string
}

type ObjectPool struct {
	pool *sync.Pool
}

func NewObjectPoolSize(bufSize int64) *ObjectPool {
	return &ObjectPool{
		pool: &sync.Pool{
			New: func() any {
				return &Object{
					compressedData: bytes.NewBuffer(make([]byte, bufSize)),
				}
			},
		},
	}
}

func NewObjectPool() *ObjectPool {
	return NewObjectPoolSize(defaultBufSize)
}

func (o *ObjectPool) New(path string, info os.FileInfo, level int) (*Object, error) {
	obj := o.pool.Get().(*Object)
	return obj, obj.Reset(path, info, level)
}

func (o *ObjectPool) Put(obj *Object) {
	o.pool.Put(obj)
}

func (o *Object) Reset(path string, info os.FileInfo, level int) error {
	if path == "" || info == nil {
		return errors.New("invalid path or info")
	}

	if err := validLevel(level); err != nil {
		return err
	}

	var (
		hdr  *FileHeader
		link string
		err  error
	)

	if IsSymlink(info.Mode()) {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}

	if hdr, err = zip.FileInfoHeader(info); err != nil {
		return err
	}
	hdr.Name = HeaderName(path)
	if o.compressor == nil {
		o.compressor, err = flate.NewWriter(o, level)
		if err != nil {
			return err
		}
	} else {
		o.compressor.Reset(o)
	}

	o.Path = path
	o.Info = info
	o.header = hdr
	o.compressedData.Reset()
	o.overflow = nil
	o.written = 0
	o.link = link
	if level > 6 {
		o.compressMinSize = 44
	} else {
		o.compressMinSize = 128
	}
	return nil
}

func (o *Object) Write(p []byte) (n int, err error) {
	totalLen := len(p)
	if o.compressedData.Available() != 0 {
		maxWriteable := min(o.compressedData.Available(), totalLen)
		o.written += uint64(maxWriteable)
		o.compressedData.Write(p[:maxWriteable])
		p = p[maxWriteable:]
	}
	if len(p) > 0 {
		if o.overflow == nil {
			if o.overflow, err = os.CreateTemp(o.Root, overflowPrefix); err != nil {
				return len(p), fmt.Errorf("create temporary file: %w", err)
			}
		}
		if n, err = o.overflow.Write(p); err != nil {
			return len(p), fmt.Errorf("write temporary file for %q: %w", o.Path, err)
		}

		o.written += uint64(n)
	}
	return totalLen, nil
}

func (o *Object) prepareHeader() error {
	utf8ValidName, utf8RequireName := detectUTF8(o.header.Name)
	utf8ValidComment, utf8RequireComment := detectUTF8(o.header.Comment)
	switch {
	case o.header.NonUTF8:
		o.header.Flags &^= 0x800
	case (utf8RequireName || utf8RequireComment) && (utf8ValidName && utf8ValidComment):
		o.header.Flags |= 0x800
	}

	if !o.header.Modified.IsZero() {
		// Use "extended timestamp" format since this is what Info-ZIP uses.
		// Nearly every major ZIP implementation uses a different format,
		// but at least most seem to be able to understand the other formats.
		//
		// This format happens to be identical for both local and central header
		// if modification time is the only timestamp being encoded.
		var mbuf [9]byte // 2*SizeOf(uint16) + SizeOf(uint8) + SizeOf(uint32)
		eb := writeBuf(mbuf[:])
		eb.uint16(extTimeExtraID)
		eb.uint16(5)                                // Size: SizeOf(uint8) + SizeOf(uint32)
		eb.uint8(1)                                 // Flags: ModTime
		eb.uint32(uint32(o.header.Modified.Unix())) // ModTime

		o.header.Extra = append(o.header.Extra, mbuf[:]...)
	}

	o.header.CreatorVersion = o.header.CreatorVersion&0xff00 | zipVersion20 // preserve compatibility byte
	o.header.ReaderVersion = zipVersion20
	o.header.Flags &^= 0x8 // won't write data descriptor (crc32, comp, uncomp)

	// Dir
	if o.Info.IsDir() {
		if !strings.HasSuffix(o.header.Name, "/") {
			o.header.Name += "/" // required
		}
		o.header.Method = zip.Store
		// not write
		o.header.CompressedSize64 = 0
		o.header.UncompressedSize64 = 0
		return nil
	}

	// symlink
	size := o.header.UncompressedSize64
	if o.link != "" {
		size = uint64(len(o.link))
		// reset uncompressedSize64
		o.header.UncompressedSize64 = size
	}

	// No need to compress files
	if size <= o.compressMinSize || IsCompressedFile(o.Path) {
		o.header.Method = zip.Store
	} else {
		// File
		o.header.Method = zip.Deflate
	}
	return nil
}

func (o *Object) Compress() error {
	err := o.prepareHeader()
	if err != nil {
		return err
	}

	if o.Info.IsDir() {
		return nil
	}

	switch o.header.Method {
	case zip.Store:
		err = o.store()
	case zip.Deflate:
		err = o.deflate()
	default:
		return fmt.Errorf("unknown compress method: %d", o.header.Method)
	}

	if err != nil {
		return err
	}

	return nil
}

func (o *Object) deflate() error {
	hash32 := crc32.NewIEEE()
	w := io.MultiWriter(o.compressor, hash32)
	if o.link != "" {
		bs := []byte(o.link)
		_, err := w.Write(bs)
		if err != nil {
			return err
		}
		o.header.CompressedSize64 = o.written
		o.header.CRC32 = hash32.Sum32()
		return nil
	}

	fd, err := os.Open(o.Path)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = io.Copy(w, fd)
	if err != nil {
		return err
	}
	if err = o.compressor.Close(); err != nil {
		return fmt.Errorf("close compressor for %q: %w", o.Path, err)
	}

	o.header.CompressedSize64 = o.written
	o.header.CRC32 = hash32.Sum32()
	return nil
}

func (o *Object) store() error {
	hash32 := crc32.NewIEEE()
	if o.link != "" {
		_, err := hash32.Write([]byte(o.link))
		if err != nil {
			return err
		}
		o.header.CompressedSize64 = o.header.UncompressedSize64
		o.header.CRC32 = hash32.Sum32()
		return nil
	}

	fd, err := os.Open(o.Path)
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = io.Copy(hash32, fd)
	if err != nil {
		return err
	}
	o.header.CompressedSize64 = o.header.UncompressedSize64
	o.header.CRC32 = hash32.Sum32()
	return nil
}

func (o *Object) Archive(w *Writer) error {
	cw, err := w.CreateRaw(o.header)
	if err != nil {
		return fmt.Errorf("create raw for %q: %w", o.Path, err)
	}

	if o.Info.IsDir() {
		return nil
	}

	if o.header.Method == zip.Store {
		if o.link != "" {
			if _, err = io.Copy(cw, strings.NewReader(o.link)); err != nil {
				return fmt.Errorf("store %q: %w", o.Path, err)
			}
			return nil
		}

		fd, err := os.Open(o.Path)
		if err != nil {
			return fmt.Errorf("store %q: %w", o.Path, err)
		}
		defer fd.Close()
		if _, err = io.Copy(cw, fd); err != nil {
			return fmt.Errorf("store %q: %w", o.Path, err)
		}
	} else {
		if _, err = io.Copy(cw, o.compressedData); err != nil {
			return fmt.Errorf("write compressed data for %q: %w", o.Path, err)
		}
		if o.Overflowed() {
			if _, err = o.overflow.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("seek overflow for %q: %w", o.Path, err)
			}
			if _, err = io.Copy(cw, o.overflow); err != nil {
				return fmt.Errorf("copy overflow for %q: %w", o.Path, err)
			}
		}
	}

	return nil
}

func (o *Object) Written() uint64 {
	return o.written
}

func (o *Object) Overflowed() bool {
	return o.overflow != nil
}

func (o *Object) Close() error {
	if o.Overflowed() {
		if err := o.overflow.Close(); err != nil {
			return fmt.Errorf("close overflow file: %w", err)
		}
		_ = os.Remove(o.overflow.Name())
	}
	return nil
}

func IsSymlink(mode fs.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

func validLevel(level int) error {
	if level < -2 || level > 9 {
		return fmt.Errorf("invalid compression level %d: want value in range [-2, 9]", level)
	}
	return nil
}

// compressedFormats is a (non-exhaustive) set of lowercased
// file extensions for formats that are typically already
// compressed. Compressing files that are already compressed
// is inefficient, so use this set of extensions to avoid that.
var compressedFormats = map[string]struct{}{
	".7z":   {},
	".avi":  {},
	".br":   {},
	".bz2":  {},
	".cab":  {},
	".docx": {},
	".gif":  {},
	".gz":   {},
	".jar":  {},
	".jpeg": {},
	".jpg":  {},
	".lz":   {},
	".lz4":  {},
	".lzma": {},
	".m4v":  {},
	".mov":  {},
	".mp3":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".png":  {},
	".pptx": {},
	".rar":  {},
	".sz":   {},
	".tbz2": {},
	".tgz":  {},
	".tsz":  {},
	".txz":  {},
	".xlsx": {},
	".xz":   {},
	".zip":  {},
	".zipx": {},
}

func IsCompressedFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := compressedFormats[ext]
	return ok
}

func HeaderName(path string) string {
	if runtime.GOOS == "windows" {
		path = strings.TrimPrefix(path, filepath.VolumeName(path))
	}
	path = filepath.Clean(path)
	return strings.TrimPrefix(filepath.ToSlash(path), "/")
}

func FormatName(name string) string {
	ext := filepath.Ext(name)
	if ext == ".zip" {
		return name
	}
	return name + ".zip"
}
