# 🚀 go_do_work

> A lightweight, in-memory task queue with powerful concurrency controls for Go applications.

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
    
    H --> L[Task Status Map]
    K --> L
    
    A -->|Query Status| L
    
    M[Shutdown Signal] -->|Graceful Shutdown| N[Wait for Workers]
    N -->|Complete In-flight Tasks| O[Terminate]
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
info, err := taskQueue.Status(taskID)
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

| Option | Description |
|--------|-------------|
| `WithWorkerCount(n)` | Set number of concurrent workers |
| `WithQueueSize(n)` | Set maximum queue capacity |
| `WithRetryPolicy(...)` | Configure retry behavior |
| `WithBufferSize(n)` | Set channel buffer size |

## 📖 API Reference

### Core Methods

- `Submit(ctx, task)` - Queue a task for immediate execution
- `Schedule(ctx, task, delay)` - Queue a task to run after a delay
- `ScheduleAt(ctx, task, time)` - Queue a task to run at a specific time
- `ScheduleRecurring(ctx, task, interval)` - Queue a recurring task
- `Status(taskID)` - Get current task status
- `Shutdown(ctx)` - Gracefully stop the queue

## 📄 License

MIT © [Luke Ward](https://github.com/dukebward)