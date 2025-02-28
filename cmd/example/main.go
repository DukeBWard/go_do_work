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
	// create a context that can be canceled on program termination
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received")
		cancel()
	}()

	// create the queue with 5 workers, a buffer of 100 tasks
	// and a simple retry policy
	taskQueue := queue.NewTaskQueue(
		queue.WithWorkerCount(5),
		queue.WithQueueSize(100),
		queue.WithRetryPolicy(3, time.Second, time.Minute, queue.RetryFixed, nil),
	)
	fmt.Println("Task queue started with 5 workers")

	// submit a simple task that will fail
	taskID, err := taskQueue.Submit(ctx, func(ctx context.Context) error {
		// some work here, e.g. sending an email
		fmt.Println("Sending an email...")
		// simulate error
		return errors.New("failed to send email: connection timeout")
	})
	if err != nil {
		log.Fatalf("Failed to submit task: %v", err)
	}
	fmt.Printf("Submitted task with ID: %s\n", taskID)

	// check status after a short delay
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

	// schedule a task to run after a delay
	scheduledID, err := taskQueue.Schedule(ctx, func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			fmt.Println("Running delayed task...")
			return nil
		}
	}, 5*time.Second)
	if err != nil {
		log.Printf("Failed to schedule task: %v", err)
	} else {
		fmt.Printf("Scheduled delayed task with ID: %s\n", scheduledID)
	}

	// schedule a task to run at a specific time (5 seconds from now)
	executeAt := time.Now().Add(5 * time.Second)
	scheduledAtID, err := taskQueue.ScheduleAt(ctx, func(ctx context.Context) error {
		fmt.Println("Running task scheduled for specific time:", executeAt.Format(time.RFC3339))
		return nil
	}, executeAt)
	if err != nil {
		log.Printf("Failed to schedule task at specific time: %v", err)
	} else {
		fmt.Printf("Scheduled task at %s with ID: %s\n", executeAt.Format(time.RFC3339), scheduledAtID)
	}

	// schedule a recurring task
	recurringID, err := taskQueue.ScheduleRecurring(ctx, func(ctx context.Context) error {
		fmt.Println("Running recurring task at:", time.Now().Format(time.RFC3339))
		return nil
	}, 2*time.Second)
	if err != nil {
		log.Printf("Failed to schedule recurring task: %v", err)
	} else {
		fmt.Printf("Scheduled recurring task with ID: %s\n", recurringID)
	}

	// let the program run for a bit to allow tasks to complete
	select {
	case <-ctx.Done():
		fmt.Println("Context canceled, shutting down...")
	case <-time.After(15 * time.Second):
		fmt.Println("Timeout reached, shutting down...")
	}

	// gracefully shutdown with a timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	if err := taskQueue.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	} else {
		fmt.Println("Task queue shut down cleanly")
	}
}
