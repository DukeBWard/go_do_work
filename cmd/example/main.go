package example

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dukebward/go_do_work/internal/queue"
)

func main() {
	// Create the queue with, say, 5 workers, a buffer of 100 tasks
	// and default retry policy.
	taskQueue := queue.NewTaskQueue(
		queue.WithWorkerCount(5),
		queue.WithQueueSize(100),
		queue.WithRetryPolicy(3, time.Second),
	)

	// Submit a simple task
	taskID, _ := taskQueue.Submit(context.Background(), func(ctx context.Context) error {
		// Some work here, e.g. sending an email
		fmt.Println("Sending an email...")
		// Simulate error
		return errors.New("oops, failed to send email")
	})

	// We could check status later
	time.Sleep(2 * time.Second)
	info, _ := taskQueue.Status(taskID)
	fmt.Printf("Task status: %s, attempts so far: %d\n", info.Status, info.Attempts)

	// Schedule a task in the future
	_, _ = taskQueue.Schedule(context.Background(), func(ctx context.Context) error {
		fmt.Println("This runs 10 seconds later.")
		return nil
	}, 10*time.Second)

	// Let the program run for a bit
	time.Sleep(12 * time.Second)

	// Now gracefully shutdown
	_ = taskQueue.Shutdown(context.Background())
	fmt.Println("Exited cleanly.")
}
