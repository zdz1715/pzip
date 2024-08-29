package pzip

import (
	"reflect"
	"testing"
)

func TestHeaderPrepare(t *testing.T) {
	for i := 0; i < 1000; i++ {
		h := &header{
			FileHeader: &FileHeader{
				CompressedSize64:   uint64(i),
				UncompressedSize64: uint64(i),
			},
			offset: uint32max + uint64(i),
		}
		h1 := &header{
			FileHeader: &FileHeader{
				CompressedSize64:   uint64(i),
				UncompressedSize64: uint64(i),
			},
			offset: uint32max + uint64(i),
		}
		headerPrepareByAppend(h)
		headerPrepareByWriteBuf(h1)

		if !reflect.DeepEqual(h, h1) {
			t.Errorf("header prepare fail: got %v, want %v", h, h1)
		}
	}
}

func BenchmarkHeaderPrepare(b *testing.B) {
	b.Run("HeaderPrepareByAppend", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			h := &header{
				FileHeader: &FileHeader{
					CompressedSize64:   uint32max + uint64(i),
					UncompressedSize64: uint32max + uint64(i),
				},
				offset: uint32max + uint64(i),
			}
			headerPrepareByAppend(h)
		}
	})
	b.Run("HeaderPrepareByWriteBuf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			h := &header{
				FileHeader: &FileHeader{
					CompressedSize64:   uint32max + uint64(i),
					UncompressedSize64: uint32max + uint64(i),
				},
				offset: uint32max + uint64(i),
			}

			headerPrepareByWriteBuf(h)
		}
	})
}
