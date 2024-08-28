package pzip

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"
)

type testdata struct {
	sn int
}

func TestNewFailFastWorker(t *testing.T) {

	t.Run("single worker", func(t *testing.T) {
		w := NewFailFastWorker[testdata](func(params *testdata) error {
			fmt.Println(params.sn)
			return nil
		}, 1, 1)

		w.Start(context.Background())

		for i := 0; i < 10; i++ {
			if err := w.Submit(&testdata{sn: i}); err != nil {
				t.Fatal(err)
			}
		}

		if err := w.Wait(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("num cpus of worker", func(t *testing.T) {
		w := NewFailFastWorker[testdata](func(params *testdata) error {
			fmt.Println(params.sn)
			return nil
		}, runtime.GOMAXPROCS(0), runtime.GOMAXPROCS(0))

		w.Start(context.Background())

		for i := 0; i < 10; i++ {
			if err := w.Submit(&testdata{sn: i}); err != nil {
				t.Fatal(err)
			}
		}

		if err := w.Wait(); err != nil {
			t.Fatal(err)
		}
	})

	var wantErr = errors.New("it is error")

	t.Run("single worker has error", func(t *testing.T) {
		w := NewFailFastWorker[testdata](func(params *testdata) error {
			if params.sn == 5 {
				fmt.Println(params.sn, " -> ", wantErr)
				return wantErr
			}
			fmt.Println(params.sn)
			return nil
		}, 1, 1)

		w.Start(context.Background())

		for i := 0; i < 10; i++ {
			if err := w.Submit(&testdata{sn: i}); err != nil {
				if !errors.Is(err, wantErr) {
					t.Fatal(err)
				}
				break
			}
		}

		if err := w.Wait(); err != nil && !errors.Is(err, wantErr) {
			t.Fatal(err)
		}
	})

	t.Run("num cpus of worker has error", func(t *testing.T) {
		w := NewFailFastWorker[testdata](func(params *testdata) error {
			if params.sn == 5 {
				fmt.Println(params.sn, " -> ", wantErr)
				return wantErr
			}
			fmt.Println(params.sn)
			return nil
		}, runtime.GOMAXPROCS(0), 1)

		w.Start(context.Background())

		for i := 0; i < 10; i++ {
			if err := w.Submit(&testdata{sn: i}); err != nil {
				if !errors.Is(err, wantErr) {
					t.Fatal(err)
				}
				break
			}
		}

		if err := w.Wait(); err != nil && !errors.Is(err, wantErr) {
			t.Fatal(err)
		}
	})
}
