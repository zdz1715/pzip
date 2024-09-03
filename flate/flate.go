package flate

import (
	"compress/flate"
	"io"

	fastflate "github.com/klauspost/compress/flate"
)

// github.com/klauspost/compress/flate level 8
// unizp 6.00
// error:  invalid compressed data to inflate (incomplete d-tree)
//import "github.com/klauspost/compress/flate"

//type Writer = flate.Writer

var NewWriter = flate.NewWriter
var NewFastWriter = fastflate.NewWriter

type Writer interface {
	Write(data []byte) (n int, err error)
	Reset(dst io.Writer)
	Flush() error
	Close() error
}

type NewWriterFunc func(w io.Writer, level int) (Writer, error)
