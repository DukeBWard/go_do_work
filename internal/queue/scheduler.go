package queue

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// schedule submits a task to be executed after a specified delay
func (q *TaskQueue) Schedule(ctx context.Context, task TaskFunc, delay time.Duration) (string, error) {
	taskID := uuid.New().String()

	// initialize task status for scheduled task
	q.initTaskStatus(taskID, "scheduled")

	go func() {
		select {
		case <-ctx.Done():
			// context was canceled
			q.updateTaskStatus(taskID, "canceled", 0, ctx.Err())
			return
		case <-time.After(delay):
			// delay has elapsed, submit the task to the queue
			_, err := q.Submit(ctx, task)
			if err != nil {
				q.updateTaskStatus(taskID, "failed", 0, err)
			}
		}
	}()

	return taskID, nil
}

// scheduleat submits a task to be executed at a specific time
func (q *TaskQueue) ScheduleAt(ctx context.Context, task TaskFunc, executeAt time.Time) (string, error) {
	// calculate delay until the specified time
	delay := time.Until(executeAt)
	if delay < 0 {
		// if the time is in the past, execute immediately
		return q.Submit(ctx, task)
	}
	
	return q.Schedule(ctx, task, delay)
}

// schedulerecurring submits a task to be executed repeatedly at the specified interval
func (q *TaskQueue) ScheduleRecurring(ctx context.Context, task TaskFunc, interval time.Duration) (string, error) {
	taskID := uuid.New().String()
	
	// initialize task status for recurring task
	q.initTaskStatus(taskID, "recurring")
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				q.updateTaskStatus(taskID, "canceled", 0, ctx.Err())
				return
			case <-ticker.C:
				// execute the task on each tick
				_, err := q.Submit(ctx, task)
				if err != nil {
					// log error but continue scheduling
					// we don't update the recurring task status to failed
					// as the recurring schedule itself is still active
				}
			}
		}
	}()
	
	return taskID, nil
}