package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dukebward/go_do_work/internal/queue"
)

func main() {
	// Create a context that can be canceled on program termination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received")
		cancel()
	}()

	// Create the queue with 5 workers, a buffer of 100 tasks
	// and a simple retry policy
	taskQueue := queue.NewTaskQueue(
		queue.WithWorkerCount(5),
		queue.WithQueueSize(100),
		queue.WithRetryPolicy(3, time.Second, time.Minute, queue.RetryFixed, nil),
	)
	fmt.Println("Task queue started with 5 workers")

	// Submit a simple task that will fail
	taskID, err := taskQueue.Submit(ctx, func(ctx context.Context) error {
		// Some work here, e.g. sending an email
		fmt.Println("Sending an email...")
		// Simulate error
		return errors.New("failed to send email: connection timeout")
	})
	if err != nil {
		log.Fatalf("Failed to submit task: %v", err)
	}
	fmt.Printf("Submitted task with ID: %s\n", taskID)

	// Check status after a short delay
	time.Sleep(2 * time.Second)
	info, err := taskQueue.Status(taskID)
	if err != nil {
		log.Printf("Failed to get task status: %v", err)
	} else {
		fmt.Printf("Task status: %s, attempts so far: %d\n", info.Status, info.Attempts)
		if info.LastError != nil {
			fmt.Printf("Last error: %v\n", info.LastError)
		}
	}

	// Schedule a task to run in the future
	scheduledID, err := taskQueue.Schedule(ctx, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			fmt.Println("Running scheduled task...")
			return nil
		}
	}, 5*time.Second)
	if err != nil {
		log.Printf("Failed to schedule task: %v", err)
	} else {
		fmt.Printf("Scheduled task with ID: %s\n", scheduledID)
	}

	// Let the program run for a bit to allow tasks to complete
	select {
	case <-ctx.Done():
		fmt.Println("Context canceled, shutting down...")
	case <-time.After(10 * time.Second):
		fmt.Println("Timeout reached, shutting down...")
	}

	// Gracefully shutdown with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	if err := taskQueue.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	} else {
		fmt.Println("Task queue shut down cleanly")
	}
}
