# 🚀 go_do_work

> A lightweight task queue with powerful concurrency controls for Go applications, with optional Redis persistence.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8.svg)](https://golang.org/doc/go1.21)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## 📋 Installation

```bash
go get github.com/dukebward/go_do_work
```

## ✨ Features

- **🔄 Concurrent Processing** - Configurable worker pool for parallel task execution
- **⏰ Flexible Scheduling** - Run tasks immediately, after a delay, or on a recurring schedule
- **🔁 Smart Retries** - Automatic retries with customizable backoff strategies
- **🔍 Task Tracking** - Monitor task status, attempts, and errors
- **🛑 Graceful Shutdown** - Clean termination with in-flight task completion
- **💾 Storage Options** - In-memory (default) or Redis-backed persistence (experimental)

## 📊 Architecture

```
           ┌─────────────┐
 Submit -> │ Task Queue  │ -> Worker Pool (N goroutines)
           └─────────────┘
                  │
                  ▼
         ┌─────────────────┐
         │ Task Processing │ -> Retry Logic -> Status Tracking
         └─────────────────┘
                  │
                  ▼
         ┌─────────────────┐
         │ Storage Backend │ -> In-Memory or Redis
         └─────────────────┘
```

### 🔄 Workflow Diagram

```mermaid
graph TD
    A[Client Code] -->|Submit| B[TaskQueue]
    A -->|Schedule| C[Scheduler]
    A -->|ScheduleAt| C
    A -->|ScheduleRecurring| C
    
    C -->|After Delay| B
    C -->|At Specific Time| B
    C -->|Recurring Interval| B
    
    B -->|Enqueue Task| D[Task Channel]
    D -->|Dequeue| E[Worker Pool]
    
    subgraph "Worker Pool"
        E -->|Process| F1[Worker 1]
        E -->|Process| F2[Worker 2]
        E -->|Process| F3[Worker N]
    end
    
    F1 -->|Execute| G[Task Execution]
    F2 -->|Execute| G
    F3 -->|Execute| G
    
    G -->|Success| H[Update Status: Completed]
    G -->|Failure| I[Retry Logic]
    
    I -->|Retry Attempts Remaining| J[Calculate Backoff]
    J -->|Wait| G
    
    I -->|Max Retries Exceeded| K[Update Status: Failed]
    
    subgraph "Storage Options"
        L1[In-Memory Map]
        L2[Redis Storage]
    end
    
    H --> L1
    H --> L2
    K --> L1
    K --> L2
    
    L1 --> M[Task Status]
    L2 --> M
    
    A -->|Query Status| M
    
    N[Shutdown Signal] -->|Graceful Shutdown| O[Wait for Workers]
    O -->|Complete In-flight Tasks| P[Terminate]
```

## 🚀 Quick Start

```go
// Create a task queue with 5 workers
taskQueue := queue.NewTaskQueue(
    queue.WithWorkerCount(5),
    queue.WithRetryPolicy(3, time.Second, time.Minute, queue.RetryFixed, nil),
)

// Submit a task
taskID, err := taskQueue.Submit(ctx, func(ctx context.Context) error {
    // Your task logic here
    return nil
})

// Check task status
info, err := taskQueue.Status(ctx, taskID)
fmt.Printf("Task status: %s\n", info.Status)

// Schedule a task to run after delay
scheduledID, err := taskQueue.Schedule(ctx, func(ctx context.Context) error {
    // Delayed task logic
    return nil
}, 5*time.Second)

// Graceful shutdown
err := taskQueue.Shutdown(ctx)
```

## 🔧 Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithWorkerCount(n)` | Set number of concurrent workers | 1 |
| `WithQueueSize(n)` | Set maximum queue capacity | 100 |
| `WithBufferSize(n)` | Set channel buffer size | 100 |
| `WithRetryPolicy(maxRetries, baseDelay, maxDelay, strategy, shouldRetry)` | Configure retry behavior | See below |
| `WithStorage(storage)` | Set custom storage backend | In-memory |

### Retry Policy Configuration

The retry policy can be customized using the `WithRetryPolicy` option with the following parameters:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `maxRetries` | Maximum number of retry attempts | 3 |
| `baseDelay` | Initial delay between retries | 1 second |
| `maxDelay` | Maximum delay between retries | 1 minute |
| `strategy` | Retry backoff strategy | `RetryFixed` |
| `shouldRetry` | Function that determines if an error should be retried | All errors are retried |

#### Available Retry Strategies

| Strategy | Description |
|----------|-------------|
| `RetryFixed` | Uses a fixed delay between retry attempts |
| `RetryExponential` | Increases delay exponentially between retry attempts |
| `RetryLinear` | Increases delay linearly between retry attempts |
| `RetryImmediate` | Retries immediately without any delay |

### Storage Options

By default, the task queue uses in-memory storage. For persistence and distributed scenarios, you can use the experimental Redis storage backend:

```go
// Create Redis storage
redisStorage, err := queue.NewRedisTaskStorage(
    "localhost:6379", // Redis address
    "",               // Password (empty for none)
    0,                // Database number
)
if err != nil {
    log.Fatalf("Failed to create Redis storage: %v", err)
}

// Create task queue with Redis storage
taskQueue := queue.NewTaskQueue(
    queue.WithWorkerCount(5),
    queue.WithStorage(redisStorage),
)
```

#### Redis Storage Benefits

- **Persistence**: Tasks survive application restarts
- **Distribution**: Multiple application instances can share the same queue
- **Scalability**: Easily scale workers across multiple processes or machines
- **Monitoring**: Inspect queue state using Redis tools

> **Note**: Redis storage is currently experimental and the API may change in future releases.

#### Example Configuration

```go
// Create a task queue with custom configuration
taskQueue := queue.NewTaskQueue(
    // Set 5 concurrent workers
    queue.WithWorkerCount(5),
    
    // Set queue capacity to 200 tasks
    queue.WithQueueSize(200),
    
    // Set channel buffer to 150
    queue.WithBufferSize(150),
    
    // Configure retry policy with exponential backoff
    queue.WithRetryPolicy(
        5,                      // 5 max retries
        500*time.Millisecond,   // 500ms base delay
        30*time.Second,         // 30s max delay
        queue.RetryExponential, // Exponential backoff
        func(err error) bool {  // Custom retry condition
            // Only retry specific errors
            return errors.Is(err, io.ErrUnexpectedEOF) || 
                   errors.Is(err, context.DeadlineExceeded)
        },
    ),
    
    // Optional: Use Redis storage
    queue.WithStorage(redisStorage),
)
```

## 📖 API Reference

### Core Methods

- `Submit(ctx, task)` - Queue a task for immediate execution
- `Schedule(ctx, task, delay)` - Queue a task to run after a delay
- `ScheduleAt(ctx, task, time)` - Queue a task to run at a specific time
- `ScheduleRecurring(ctx, task, interval)` - Queue a recurring task
- `Status(ctx, taskID)` - Get current task status
- `Shutdown(ctx)` - Gracefully stop the queue

## 📄 License

MIT © [Luke Ward](https://github.com/dukebward)