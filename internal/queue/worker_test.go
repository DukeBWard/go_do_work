package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// tests that workers process tasks correctly
func TestWorker_ProcessTask(t *testing.T) {
	q := NewTaskQueue(WithWorkerCount(2))

	var wg sync.WaitGroup
	wg.Add(5) // we'll submit 5 tasks

	// submit multiple tasks to test worker concurrency
	for i := 0; i < 5; i++ {
		task := func(ctx context.Context) error {
			defer wg.Done()
			return nil
		}

		_, err := q.Submit(context.Background(), task)
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}

	// wait for all tasks to complete with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// all tasks completed
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for tasks to complete")
	}

	// shutdown the queue
	err := q.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown queue: %v", err)
	}
}

// tests that workers handle task errors correctly
func TestWorker_HandleTaskError(t *testing.T) {
	q := NewTaskQueue(
		WithWorkerCount(1),
		WithRetryPolicy(0, 0, 0, RetryImmediate, nil), // no retries
	)

	// create a task that will fail
	errTest := errors.New("test error")
	taskDone := make(chan struct{})

	task := func(ctx context.Context) error {
		defer close(taskDone)
		return errTest
	}

	// submit the failing task
	id, err := q.Submit(context.Background(), task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// wait for task to complete
	select {
	case <-taskDone:
		// task executed
	case <-time.After(2 * time.Second):
		t.Fatal("Task execution timed out")
	}

	// add a small delay to ensure task status is updated
	time.Sleep(200 * time.Millisecond)

	// check task status - should be failed
	status, err := q.Status(id)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}

	if status.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", status.Status)
	}

	if status.LastError == nil || status.LastError.Error() != errTest.Error() {
		t.Errorf("Expected error '%v', got '%v'", errTest, status.LastError)
	}

	// shutdown the queue
	err = q.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown queue: %v", err)
	}
}
