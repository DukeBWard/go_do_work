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

type TaskFunc func(ctx context.Context) error

type Options struct {
	workerCount int
	queueSize   int
	bufferSize  int
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

// constructor
func NewTaskQueue(opts ...Option) *TaskQueue {
	options := &Options{
		workerCount: 1,
		queueSize:   100,
		bufferSize:  100,
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

	// initialize task status before submitting
	q.initTaskStatus(taskID, "queued")

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
