package queue

import (
	"context"
	"time"
)

type TaskQueue struct {
	workerCount int
	queueSize   int
	bufferSize  int
	retryPolicy RetryPolicy
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
