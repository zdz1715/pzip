package pzip

import "github.com/klauspost/compress/zip"

type ReadCloser = zip.ReadCloser
type File = zip.File

var OpenReader = zip.OpenReader
