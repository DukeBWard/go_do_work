package queue

import (
	"context"
	"fmt"
	"time"
)

// a worker routine

func (q *TaskQueue) worker(id int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.done:
			// queue has received done
			return
		case task, ok := <-q.tasks:
			if !ok {
				// channel was closed or no tasks
				return
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
		q.updateTaskStatus(task.ID, "failed", nil, fmt.Errorf("invalid task type"))
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

// retry logic for failed tasks
func (q *TaskQueue) handleFailedTask(ctx context.Context, task Task, taskFunc TaskFunc, err error, attempt int) {
	// get the task info and retry policy object
	maxRetries := q.opts.retryPolicy.maxRetries
	backoff := q.opts.retryPolicy.backoff

	// check if we should retry
	if attempt <= maxRetries {
		// update status
		q.updateTaskStatus(task.ID, "retrying", attempt, err)

		// calculate the backoff duration (maybe do exponential)
		retryDelay := backoff * time.Duration(attempt)

		// schedule the retry
		go func() {
			select {
			case <-time.After(retryDelay):
				// try again
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				// if no error, all good
				newErr := taskFunc(ctx)
				if newErr == nil {
					q.updateTaskStatus(task.ID, "completed", attempt+1, nil)
					return
				}

				//recurse and handle the failure with increment attempt
				q.handleFailedTask(ctx, task, taskFunc, newErr, attempt+1)
			case <-q.done:
				// queue is done
				q.updateTaskStatus(task.ID, "canceled", attempt, err)
				return
			}
		}()
	} else {
		// exhausted all retries
		q.updateTaskStatus(task.ID, "failed", attempt, err)
	}
}

// update the status of a task i nthe task status map
func (q *TaskQueue) updateTaskStatus(taskID, status string, attemts int, lastError error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.taskStatus[taskID] = TaskInfo{
		ID:        taskID,
		Status:    status,
		Attempts:  attemts,
		LastError: lastError,
	}
}
