# go_do_work
In-Memory Task Queue with Concurrency Controls

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