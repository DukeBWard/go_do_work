package queue

import (
	"context"
	"errors"
	"fmt"
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
	storage    TaskStorage
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

type TaskFunc func(ctx context.Context) error

type Options struct {
	workerCount int
	queueSize   int
	bufferSize  int
	storage     TaskStorage
	retryPolicy RetryPolicy
}

// option defines a function type for configuring taskqueue
type Option func(*Options)

// withworkercount sets the number of workers
func WithWorkerCount(count int) Option {
	return func(o *Options) {
		if count > 0 {
			o.workerCount = count
		}
	}
}

// withqueuesize sets the queue size
func WithQueueSize(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.queueSize = size
		}
	}
}

// withbuffersize sets the buffer size
func WithBufferSize(size int) Option {
	return func(o *Options) {
		if size > 0 {
			o.bufferSize = size
		}
	}
}

// withretrypolicy sets the retry policy
func WithRetryPolicy(maxRetries int, baseDelay time.Duration, maxDelay time.Duration, strategy RetryStrategy, shouldRetry func(error) bool) Option {
	return func(o *Options) {
		o.retryPolicy = RetryPolicy{
			MaxRetries:  maxRetries,
			BaseDelay:   baseDelay,
			MaxDelay:    maxDelay,
			Strategy:    strategy,
			ShouldRetry: shouldRetry,
		}
	}
}

// withstorage sets the storage
func WithStorage(storage TaskStorage) Option {
	return func(o *Options) {
		o.storage = storage
	}
}

// constructor
func NewTaskQueue(opts ...Option) *TaskQueue {
	options := &Options{
		workerCount: 1,
		queueSize:   100,
		bufferSize:  100,
		storage:     nil, // Default to in-memory
		retryPolicy: RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  1 * time.Second,
			MaxDelay:   1 * time.Minute,
			Strategy:   RetryFixed,
			ShouldRetry: func(err error) bool {
				// by default retry all errors
				return err != nil
			},
		},
	}

	// apply all options
	for _, opt := range opts {
		opt(options)
	}

	queue := &TaskQueue{
		opts:       options,
		tasks:      make(chan Task, options.bufferSize),
		done:       make(chan struct{}),
		mu:         sync.Mutex{},
		taskStatus: make(map[string]TaskInfo),
		storage:    options.storage,
	}

	// automatically start workers
	queue.startWorkers()

	return queue
}

func (q *TaskQueue) Submit(ctx context.Context, task TaskFunc) (string, error) {
	taskID := uuid.New().String()

	newTask := Task{
		ID:      taskID,
		Payload: task,
	}

	// Initialize task status
	info := TaskInfo{
		ID:       taskID,
		Status:   "queued",
		Attempts: 0,
	}

	// Use storage if available
	if q.storage != nil {
		if err := q.storage.SaveTask(ctx, taskID, info); err != nil {
			return "", err
		}

		if err := q.storage.EnqueueTask(ctx, newTask); err != nil {
			return "", err
		}

		return taskID, nil
	}

	// Fall back to in-memory implementation
	q.mu.Lock()
	q.taskStatus[taskID] = info
	q.mu.Unlock()

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

	// wait for all current tasks to finish with a timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	var shutdownErr error

	// use a timeout of 5 seconds for waiting on workers to finish
	// this prevents shutdown from hanging indefinitely if workers are stuck
	// especially important when using redis storage with blocking operations
	shutdownTimeout := time.After(5 * time.Second)

	select {
	case <-done:
		// if shutdown successful
		// close storage if available
		if q.storage != nil {
			if err := q.storage.Close(); err != nil {
				shutdownErr = fmt.Errorf("error closing storage: %w", err)
			}
		}
	case <-shutdownTimeout:
		// timeout reached, force shutdown
		shutdownErr = errors.New("shutdown timed out after 5 seconds")
	case <-ctx.Done():
		// have reached context deadline
		shutdownErr = errors.New("shutdown failure or timeout")
	}

	return shutdownErr
}

func (q *TaskQueue) Status(ctx context.Context, id string) (TaskInfo, error) {
	if q.storage != nil {
		return q.storage.GetTask(ctx, id)
	}

	// Fall back to in-memory implementation
	q.mu.Lock()
	defer q.mu.Unlock()

	taskInfo, ok := q.taskStatus[id]
	if !ok {
		return TaskInfo{}, errors.New("task ID not found")
	}
	return taskInfo, nil
}

// inittaskstatus initializes a task status in the status map (private method)
func (q *TaskQueue) initTaskStatus(taskID, status string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.taskStatus[taskID] = TaskInfo{
		ID:       taskID,
		Status:   status,
		Attempts: 0,
	}
}
