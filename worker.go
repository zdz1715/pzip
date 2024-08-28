package pzip

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

const (
	// OPENED represents that the pool is opened.
	OPENED = iota + 1
	// CLOSED represents that the pool is closed.
	CLOSED
)

var (
	ErrWorkerClosed    = errors.New("this worker has been closed")
	ErrWorkerNotOpened = errors.New("this worker has not been opened")
)

type Executor[T any] func(params *T) error

type FailFastWorker[T any] struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel func(err error)

	// state is used to notice the pool to closed itself.
	state       int32
	parallelism int
	capacity    int
	tasks       chan *T
	err         error
	errOnce     sync.Once
	executor    Executor[T]
}

func NewFailFastWorker[T any](executor Executor[T], parallelism int, Capacity int) *FailFastWorker[T] {
	return &FailFastWorker[T]{
		tasks:       make(chan *T, Capacity),
		executor:    executor,
		parallelism: parallelism,
		capacity:    Capacity,
	}
}

func (fw *FailFastWorker[T]) reset(ctx context.Context) {
	atomic.StoreInt32(&fw.state, OPENED)
	fw.tasks = make(chan *T, fw.capacity)
	fw.ctx, fw.cancel = context.WithCancelCause(ctx)
	fw.err = nil
	fw.errOnce = sync.Once{}
}

func (fw *FailFastWorker[T]) Start(ctx context.Context) {
	fw.reset(ctx)

	for i := 0; i < fw.parallelism; i++ {
		fw.wg.Add(1)
		go func() {
			defer fw.wg.Done()

			if err := fw.exec(); err != nil {
				fw.errOnce.Do(func() {
					fw.err = err
					if fw.cancel != nil {
						fw.cancel(fw.err)
					}
				})
			}
		}()
	}
}

func (fw *FailFastWorker[T]) exec() error {
	for {
		select {
		case <-fw.ctx.Done():
			return fw.ctx.Err()
		case t, ok := <-fw.tasks:
			if !ok {
				return nil
			}
			if err := fw.executor(t); err != nil {
				return err
			}
		}
	}
}

// Len returns the number of tasks that are waiting to be processed
func (fw *FailFastWorker[T]) Len() int {
	return len(fw.tasks)
}

func (fw *FailFastWorker[T]) Wait() error {
	if fw.IsClosed() {
		return ErrWorkerClosed
	}
	if !fw.IsOpened() {
		return ErrWorkerNotOpened
	}

	// close
	close(fw.tasks)

	fw.wg.Wait()
	atomic.StoreInt32(&fw.state, CLOSED)
	return fw.err
}

// IsClosed indicates whether the worker is closed.
func (fw *FailFastWorker[T]) IsClosed() bool {
	return atomic.LoadInt32(&fw.state) == CLOSED
}

// IsOpened indicates whether the worker is opened.
func (fw *FailFastWorker[T]) IsOpened() bool {
	return atomic.LoadInt32(&fw.state) == OPENED
}

func (fw *FailFastWorker[T]) Submit(task *T) error {
	if fw.err != nil {
		return fw.err
	}

	if !fw.IsOpened() {
		return ErrWorkerNotOpened
	}

	if fw.IsClosed() {
		return ErrWorkerClosed
	}

	select {
	case fw.tasks <- task:
		// Task submitted successfully
	case <-fw.ctx.Done():
		if fw.err != nil {
			return fw.err
		}
		return fw.ctx.Err()
	}
	return nil
}
