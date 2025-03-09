package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupMockRedis creates a mock Redis server for testing
func setupMockRedis(t *testing.T) (*miniredis.Miniredis, *RedisTaskStorage) {
	// Create a new mock Redis server
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to create mock Redis server: %v", err)
	}

	// Create a Redis client that connects to the mock server
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// Create a RedisTaskStorage with the mock client
	storage := &RedisTaskStorage{
		client:     client,
		queueKey:   "test:queue",
		taskPrefix: "test:task:",
		taskFuncs:  make(map[string]interface{}),
		mu:         sync.RWMutex{},
	}

	return s, storage
}

func TestRedisTaskStorage_SaveAndGetTask(t *testing.T) {
	// Setup mock Redis
	s, storage := setupMockRedis(t)
	defer s.Close()

	// Create a test task info
	taskInfo := TaskInfo{
		ID:        "test-task-1",
		Status:    "queued",
		Attempts:  0,
		LastError: nil,
	}

	// Save the task
	ctx := context.Background()
	err := storage.SaveTask(ctx, taskInfo.ID, taskInfo)
	if err != nil {
		t.Fatalf("Failed to save task: %v", err)
	}

	// Get the task
	retrievedInfo, err := storage.GetTask(ctx, taskInfo.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	// Verify the task info
	if retrievedInfo.ID != taskInfo.ID {
		t.Errorf("Expected task ID %s, got %s", taskInfo.ID, retrievedInfo.ID)
	}
	if retrievedInfo.Status != taskInfo.Status {
		t.Errorf("Expected task status %s, got %s", taskInfo.Status, retrievedInfo.Status)
	}
	if retrievedInfo.Attempts != taskInfo.Attempts {
		t.Errorf("Expected task attempts %d, got %d", taskInfo.Attempts, retrievedInfo.Attempts)
	}
}

func TestRedisTaskStorage_EnqueueAndDequeueTask(t *testing.T) {
	// Setup mock Redis
	s, storage := setupMockRedis(t)
	defer s.Close()

	// Create a test task
	taskFunc := func(ctx context.Context) error { return nil }
	task := Task{
		ID:      "test-task-2",
		Payload: taskFunc,
	}

	// Enqueue the task
	ctx := context.Background()
	err := storage.EnqueueTask(ctx, task)
	if err != nil {
		t.Fatalf("Failed to enqueue task: %v", err)
	}

	// Verify the task function is stored in memory
	storage.mu.RLock()
	storedFunc, exists := storage.taskFuncs[task.ID]
	storage.mu.RUnlock()

	if !exists {
		t.Errorf("Task function not stored in memory")
	}

	if storedFunc == nil {
		t.Errorf("Stored task function is nil")
	}

	// Dequeue the task with a timeout
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	dequeuedTask, err := storage.DequeueTask(ctxWithTimeout)
	if err != nil {
		t.Fatalf("Failed to dequeue task: %v", err)
	}

	// Verify the task
	if dequeuedTask.ID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, dequeuedTask.ID)
	}

	// Verify the payload is a function
	if dequeuedTask.Payload == nil {
		t.Errorf("Dequeued task payload is nil")
	}
}

func TestRedisTaskStorage_GetNonExistentTask(t *testing.T) {
	// Setup mock Redis
	s, storage := setupMockRedis(t)
	defer s.Close()

	// Try to get a non-existent task
	ctx := context.Background()
	_, err := storage.GetTask(ctx, "non-existent-task")

	// Should return an error
	if err == nil {
		t.Error("Expected error when getting non-existent task, got nil")
	}
}

func TestTaskQueueWithRedisStorage(t *testing.T) {
	// Setup mock Redis
	s, storage := setupMockRedis(t)
	defer s.Close()

	// Create a task queue with Redis storage
	q := NewTaskQueue(
		WithWorkerCount(2),
		WithStorage(storage),
	)

	// Create a channel to signal task completion
	done := make(chan bool, 1) // Use buffered channel to avoid blocking

	// Create a simple task that signals completion
	task := func(ctx context.Context) error {
		t.Log("Task is executing")
		done <- true
		t.Log("Task completed")
		return nil
	}

	// Submit the task
	t.Log("Submitting task")
	id, err := q.Submit(context.Background(), task)
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}
	t.Logf("Task submitted with ID: %s", id)

	// Wait for task completion or timeout
	t.Log("Waiting for task completion")
	select {
	case <-done:
		t.Log("Task completed successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Task execution timed out")
	}

	// Add a small delay to ensure task status is updated
	time.Sleep(500 * time.Millisecond)

	// Check task status
	t.Log("Checking task status")
	status, err := q.Status(context.Background(), id)
	if err != nil {
		t.Fatalf("Failed to get task status: %v", err)
	}
	t.Logf("Task status: %s", status.Status)

	if status.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", status.Status)
	}

	// Shutdown the queue
	t.Log("Shutting down queue")
	err = q.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("Failed to shutdown queue: %v", err)
	}
	t.Log("Queue shutdown complete")
}
