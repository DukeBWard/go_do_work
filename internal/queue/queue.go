package queue

import (
	"context"
	"time"
)

type TaskQueue struct {
	opts *Options
}

type Options struct {
	workerCount int
	queueSize   int
	bufferSize  int
	retryPolicy RetryPolicy
}

// Option defines a function type for configuring TaskQueue
type Option func(*Options)

// WithWorkerCount sets the number of workers
func WithWorkerCount(count int) Option {
	return func(o *Options) {
		if count > 0 {
			o.workerCount = count
		}
	}
}

// WithQueueSize sets the queue size
func WithQueueSize(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.queueSize = size
		}
	}
}

// WithBufferSize sets the buffer size
func WithBufferSize(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.bufferSize = size
		}
	}
}

// WithRetryPolicy sets the retry policy
func WithRetryPolicy(maxRetries int, backoff time.Duration) Option {
	return func(o *Options) {
		o.retryPolicy = RetryPolicy{
			maxRetries: maxRetries,
			backoff:    backoff,
		}
	}
}

type TaskInfo struct {
	ID        string
	Status    string
	Attempts  int
	LastError error
}

type RetryPolicy struct {
	maxRetries int
	backoff    time.Duration
}

type TaskFunc func(ctx context.Context) error

// constructor
func NewTaskQueue(opts ...Option) *TaskQueue {
	options := &Options{
		workerCount: 1,
		queueSize:   100,
		bufferSize:  100,
		retryPolicy: RetryPolicy{
			maxRetries: 3,
			backoff:    1 * time.Second,
		},
	}

	// apply all options
	for _, opt := range opts {
		opt(options)
	}

	return &TaskQueue{
		opts: options,
	}

}

func (q *TaskQueue) Submit(ctx context.Context, task TaskFunc) (string, error) {

}

func (q *TaskQueue) Shutdown(ctx context.Context) error {

}

func (q *TaskQueue) Status(id string) (TaskInfo, error) {

}

func (q *TaskQueue) Schedule(ctx context.Context, task TaskFunc, delay time.Duration) (string, error) {

}
