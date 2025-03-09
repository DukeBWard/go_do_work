package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// a worker routine

func (q *TaskQueue) worker(id int) {
	defer q.wg.Done()

	// loop until the queue is closed
	// will loop the select statement until the queue is closed
	for {
		select {
		case <-q.done:
			// queue has received done
			return
		default:
			var task Task
			var err error

			if q.storage != nil {
				// use 1-second timeout for polling
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				task, err = q.storage.DequeueTask(ctx)
				cancel()

				// check for shutdown signal after each redis operation
				// this is critical to prevent workers from hanging during shutdown
				// redis operations can block, so we need to check done channel frequently
				select {
				case <-q.done:
					return
				default:
					// continue processing
				}

				if err != nil {
					// if timeout or no tasks, continue polling
					if err == context.DeadlineExceeded || err == redis.Nil {
						time.Sleep(100 * time.Millisecond)

						// check for shutdown signal after sleep
						// sleep can delay shutdown if we don't check done channel
						select {
						case <-q.done:
							return
						default:
							// continue processing
						}

						continue
					}
					// log other errors
					continue
				}
			} else {
				// In-memory implementation
				select {
				case task, ok := <-q.tasks:
					if !ok {
						return
					}
					q.processTask(task)
				case <-q.done:
					return
				}
			}

			q.processTask(task)
		}
	}
}

func (q *TaskQueue) startWorkers() {
	for i := 0; i < q.opts.workerCount; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// processes the task with the retry logic
func (q *TaskQueue) processTask(task Task) {
	// this is goes type assertion syntax
	// it attempts to extract the value of type from the interface value
	// when you write task.Payload.(TaskFunc), you're saying
	// "I believe task.Payload contains a value of type TaskFunc, please give me that value."
	taskFunc, ok := task.Payload.(TaskFunc)
	if !ok {
		// log errors
		q.updateTaskStatus(task.ID, "failed", 1, fmt.Errorf("invalid task type"))
		return
	}

	// create context for the task
	// this creates a child context from the root context with a 30 second timeout
	// this cancel that is returned is a CancelFunc and that is what is deferred call to
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// update status
	q.updateTaskStatus(task.ID, "processing", 1, nil)

	// attempt task execution
	err := taskFunc(ctx)

	if err == nil {
		// task completed successfully
		q.updateTaskStatus(task.ID, "completed", 1, nil)
		return
	}

	// handle the failed task with retry parameters
	q.handleFailedTask(ctx, task, taskFunc, err, 1)
}

// handlefailedtask implements retry logic for tasks that fail
func (q *TaskQueue) handleFailedTask(ctx context.Context, task Task, taskFunc TaskFunc, err error, attempt int) {
	// check if we should retry this error based on policy and attempts
	if ShouldRetryError(q.opts.retryPolicy, err, attempt) {
		// update status to retrying
		q.updateTaskStatus(task.ID, "retrying", attempt, err)

		// calculate delay based on the retry strategy and attempt number
		retryDelay := CalculateRetryDelay(q.opts.retryPolicy, attempt)

		// schedule retry after delay
		go func() {
			select {
			case <-time.After(retryDelay):
				// try again
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				newErr := taskFunc(ctx)
				if newErr == nil {
					// task completed successfully
					q.updateTaskStatus(task.ID, "completed", attempt+1, nil)
					return
				}

				// recursively handle the failure with incremented attempt counter
				q.handleFailedTask(context.Background(), task, taskFunc, newErr, attempt+1)

			case <-q.done:
				// queue is shutting down
				q.updateTaskStatus(task.ID, "canceled", attempt, err)
				return
			}
		}()
	} else {
		// we've exhausted all retries
		q.updateTaskStatus(task.ID, "failed", attempt, err)
	}
}

// update the status of a task in the task status map
func (q *TaskQueue) updateTaskStatus(taskID, status string, attempts int, lastError error) {
	// Create task info
	info := TaskInfo{
		ID:        taskID,
		Status:    status,
		Attempts:  attempts,
		LastError: lastError,
	}

	// Use storage if available
	if q.storage != nil {
		// Create a background context since we don't have one from the caller
		ctx := context.Background()
		err := q.storage.SaveTask(ctx, taskID, info)
		if err != nil {
			// Log error but continue - we don't want to fail the task just because
			// we couldn't update its status
			fmt.Printf("Error updating task status in storage: %v\n", err)
		}
		return
	}

	// Fall back to in-memory implementation
	q.mu.Lock()
	defer q.mu.Unlock()

	q.taskStatus[taskID] = info
}
