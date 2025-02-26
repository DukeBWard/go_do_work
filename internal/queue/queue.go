package queue

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID      string
	Payload interface{}
}

type TaskQueue struct {
	opts       *Options
	tasks      chan Task
	taskStatus map[string]TaskInfo
	done       chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
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
		opts:       options,
		tasks:      make(chan Task, options.bufferSize),
		done:       make(chan struct{}),
		mu:         sync.Mutex{},
		taskStatus: make(map[string]TaskInfo),
	}

}

func (q *TaskQueue) Submit(ctx context.Context, task TaskFunc) (string, error) {
	taskID := uuid.New().String()

	newTask := Task{
		ID:      taskID,
		Payload: task,
	}

	select {
	case q.tasks <- newTask:
		return taskID, nil
	default:
		return "", errors.New("task queue is at capacity")
	}
}

func (q *TaskQueue) Shutdown(ctx context.Context) error {
	// stop current workers of the current task queue
	close(q.done)

	// wait for all current tasks to finish
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// if shutdown successful
		return nil
	case <-ctx.Done():
		// have reached context deadline
		return errors.New("shutdown failure or timeout")
	}
}

func (q *TaskQueue) Status(id string) (TaskInfo, error) {
	// the mute will lock the current task queue
	q.mu.Lock()
	// unlock mutex at end
	defer q.mu.Unlock()

	taskInfo, ok := q.taskStatus[id]
	if !ok {
		return TaskInfo{}, errors.New("task Id was not found")
	}
	return taskInfo, nil
}

func (q *TaskQueue) Schedule(ctx context.Context, task TaskFunc, delay time.Duration) (string, error) {
	taskID := uuid.New().String()

	go func() {
		select {
		case <-ctx.Done():
			// ctx was canceled
			return
		case <-time.After(delay):
			// delay has elapsed, submit the task to the q
			q.Submit(ctx, task)
		}
	}()

	return taskID, nil
}
