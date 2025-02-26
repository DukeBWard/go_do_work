package example

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func main() {
	// Create the queue with, say, 5 workers, a buffer of 100 tasks
	// and default retry policy.
	queue := NewTaskQueue(
		WithWorkerCount(5),
		WithQueueSize(100),
		WithMaxRetries(3),
		WithExponentialBackoff(time.Second, 5*time.Minute),
	)

	// Submit a simple task
	taskID, _ := queue.Submit(func(ctx context.Context) error {
		// Some work here, e.g. sending an email
		fmt.Println("Sending an email...")
		// Simulate error
		return errors.New("oops, failed to send email")
	})

	// We could check status later
	time.Sleep(2 * time.Second)
	info, _ := queue.GetInfo(taskID)
	fmt.Printf("Task status: %s, attempts so far: %d\n", info.Status, info.Attempts)

	// Schedule a task in the future
	_, _ = queue.Schedule(func(ctx context.Context) error {
		fmt.Println("This runs 10 seconds later.")
		return nil
	}, time.Now().Add(10*time.Second))

	// Let the program run for a bit
	time.Sleep(12 * time.Second)

	// Now gracefully shutdown
	_ = queue.Shutdown(context.Background())
	fmt.Println("Exited cleanly.")
}
