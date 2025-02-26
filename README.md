# go_do_work
In-Memory Task Queue with Concurrency Controls

# Core Features

### Producer API

A function that allows the user to enqueue a “task” (which could be a function, a closure, or a structure describing the work).
Users might call something like queue.Submit(func(ctx context.Context) error { ... }), returning an ID or a future/promise for status.

### Worker Pool

A configurable number of workers (N goroutines) each reading tasks from a shared channel.
If the queue is empty, workers idle; if full, tasks are buffered or blocked until a slot is available.

### Scheduler / Delayed Queue (Optional but valuable)

Ability to schedule a task to run in the future (e.g., “run this job in 5 minutes”), using either a min-heap or a timer-based approach behind the scenes.
Internally, you might maintain a priority queue or a separate scheduling goroutine that wakes up tasks at the right time.

### Retry Logic

On failure, you can re-queue the task with a backoff (exponential or fixed).
The user configures max retries, backoff intervals, or a custom function that decides whether to retry.

### Task Lifecycle and Status

Optionally track the status of tasks: “queued,” “in progress,” “completed,” or “failed.”
Allow users to query the status of a task by ID or retrieve the final result.

### Graceful Shutdown

Provide a method like queue.Shutdown(ctx context.Context) that stops accepting new tasks and waits for in-flight tasks (within a certain timeout) to finish.
Or a forced shutdown if tasks exceed the timeout.

### Context Support

Each task can receive a context.Context for cancellation or timeout.
If the user’s app is shutting down (or if the user explicitly cancels a task), the worker should respect that context and stop work gracefully.

# Nice to haves:

### Instrumentation / Metrics

Expose metrics (e.g., Prometheus) about task rates, failures, retries, queue length, worker utilization.
Developers love having insights into what’s happening under the hood.

### Task Prioritization

Offer separate priority queues or a single queue with priority ordering, so urgent tasks jump ahead of normal ones.

### Web Dashboard / CLI

Provide a small interface to see queued tasks, in-progress tasks, and statuses. Even a simple textual UI or exposed REST endpoints can be a plus.

### Pluggable Storage

For advanced use-cases, tasks could persist to disk, or you could plug in a database to survive restarts.
This starts to become more like a “real” job scheduler, but it’s a valuable extension if you want your library to handle restarts gracefully.

```
           ┌─────────────┐
 Enqueue ->│ Task Channel │<- Spawned by the Producer
           └─────────────┘
                 ↓
          ┌────────────────┐
          │ Worker 1 (goroutine)  ──┐
          └────────────────┘         │
          ┌────────────────┐         │ concurrency-limited
          │ Worker 2 (goroutine)  ──┤
          └────────────────┘         │
                 ...                ...
          ┌────────────────┐         │
          │ Worker N (goroutine)  ──┘
          └────────────────┘
                 ↓
   ┌───────────────────────────┐
   │(Optional) Retry Logic,    │
   │    Backoff, Scheduling    │
   └───────────────────────────┘
                 ↓
          Task Completion / Result
```

# Potential Design

```go
type TaskFunc func(ctx context.Context) error

type TaskQueue interface {
    // Submit a task to be run as soon as possible.
    // Returns a unique task ID or a handle for status tracking.
    Submit(task TaskFunc) (string, error)
    
    // Like Submit, but with a scheduled time in the future (delayed task).
    Schedule(task TaskFunc, runAt time.Time) (string, error)

    // Retrieve info about a task (status, retries, errors, etc.).
    GetInfo(taskID string) (TaskInfo, error)

    // Stop accepting new tasks and gracefully shut down workers.
    Shutdown(ctx context.Context) error
}

type TaskInfo struct {
    Status   string // "queued", "running", "completed", "failed", etc.
    Attempts int
    LastError error
    // ... maybe more metadata
}

func NewTaskQueue(opts ...Option) TaskQueue {
    // Implementation...
}
```

## Worker pool concurrency could be set via options:
``` go
queue := NewTaskQueue(
    WithWorkerCount(10),
    WithRetryOptions(...),
    WithBufferSize(1000),
    // etc.
)
```