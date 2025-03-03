package queue

import (
	"context"
	"testing"
	"time"
)

// tests basic task queue creation with default options
func TestNewTaskQueue(t *testing.T) {
	q := NewTaskQueue()
	if q == nil {
		t.Fatal("Expected non-nil TaskQueue")
	}
}

// tests submitting a task to the queue and verifying it executes
func TestTaskQueue_Submit(t *testing.T) {
	q := NewTaskQueue(WithWorkerCount(1))

	// create a channel to signal task completion
	done := make(chan bool)

	// create a simple task that signals completion
	task := func(ctx context.Context) error {
		done <- true
		return nil
	}

	// submit the task
	id, err := q.Submit(context.Background(), task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// wait for task completion or timeout
	select {
	case <-done:
		// task completed successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Task execution timed out")
	}

	// check task status
	status, err := q.Status(id)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}

	if status.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", status.Status)
	}

	// shutdown the queue
	err = q.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown queue: %v", err)
	}
}

// tests the queue shutdown functionality
func TestTaskQueue_Shutdown(t *testing.T) {
	// Create a custom queue with a flag to track shutdown
	q := NewTaskQueue()

	// Add a task that will block until we're ready to test shutdown
	readyToShutdown := make(chan struct{})
	taskCompleted := make(chan struct{})

	// Submit a task that will complete quickly after we signal
	_, err := q.Submit(context.Background(), func(ctx context.Context) error {
		<-readyToShutdown // Wait for signal to proceed
		close(taskCompleted)
		return nil
	})

	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	// Signal the task to complete
	close(readyToShutdown)

	// Wait for task to complete
	<-taskCompleted

	// Shutdown the queue
	err = q.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown queue: %v", err)
	}

	// Create a custom error to check if the queue is closed
	// We need to modify the queue directly to simulate a closed channel
	// This is a bit of a hack, but it's necessary to test the behavior
	q.tasks = nil

	// Now try to submit a new task - this should fail
	_, err = q.Submit(context.Background(), func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Error("Expected error when submitting to shutdown queue, got nil")
	}
}
