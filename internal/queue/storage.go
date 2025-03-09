package queue

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// TaskStorage defines the interface for task storage backends
type TaskStorage interface {
	SaveTask(ctx context.Context, taskID string, info TaskInfo) error
	GetTask(ctx context.Context, taskID string) (TaskInfo, error)
	EnqueueTask(ctx context.Context, task Task) error
	DequeueTask(ctx context.Context) (Task, error)
	Close() error
}

// TaskReference is used to store a reference to a task function
// since we can't serialize functions to JSON
type TaskReference struct {
	ID string `json:"id"`
}

// RedisTaskStorage implements TaskStorage using Redis
type RedisTaskStorage struct {
	client     *redis.Client          // handles talking to redis server
	queueKey   string                 // unique name for our task list
	taskPrefix string                 // prefix for storing individual task info
	taskFuncs  map[string]interface{} // in-memory map of task functions
	mu         sync.RWMutex           // mutex for taskFuncs map
}

// NewRedisTaskStorage creates a new Redis-backed task storage
func NewRedisTaskStorage(addr string, password string, db int) (*RedisTaskStorage, error) {
	// create redis client
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisTaskStorage{
		client:     client,
		queueKey:   "go_do_work:queue", // name for our list of tasks
		taskPrefix: "go_do_work:task:", // prefix for individual task storage
		taskFuncs:  make(map[string]interface{}),
		mu:         sync.RWMutex{},
	}, nil
}

// SaveTask stores task info in Redis
func (r *RedisTaskStorage) SaveTask(ctx context.Context, taskID string, info TaskInfo) error {
	// convert task info to json
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// store in redis using prefix + id as key
	return r.client.Set(ctx, r.taskPrefix+taskID, data, 0).Err()
}

// GetTask retrieves task info from Redis
func (r *RedisTaskStorage) GetTask(ctx context.Context, taskID string) (TaskInfo, error) {
	// look up task by its key (prefix + id)

	data, err := r.client.Get(ctx, r.taskPrefix+taskID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return TaskInfo{}, errors.New("task not found")
		}
		return TaskInfo{}, err
	}

	// convert json back to task info
	var info TaskInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return TaskInfo{}, err
	}

	return info, nil
}

// EnqueueTask adds a task to the Redis queue
func (r *RedisTaskStorage) EnqueueTask(ctx context.Context, task Task) error {
	// Store the task function in memory
	r.mu.Lock()
	r.taskFuncs[task.ID] = task.Payload
	r.mu.Unlock()

	// Create a reference to store in Redis
	taskRef := TaskReference{
		ID: task.ID,
	}

	// convert task reference to json
	data, err := json.Marshal(taskRef)
	if err != nil {
		return err
	}

	// add to left side of list (back of queue)
	return r.client.LPush(ctx, r.queueKey, data).Err()
}

// DequeueTask removes and returns a task from the Redis queue
func (r *RedisTaskStorage) DequeueTask(ctx context.Context) (Task, error) {
	// remove from right side of list (front of queue)
	// use a 1-second timeout instead of blocking indefinitely (0)
	// this allows the worker to check for shutdown signals more frequently
	// using 0 as timeout would block forever and prevent clean shutdown
	result, err := r.client.BRPop(ctx, 1*time.Second, r.queueKey).Result()
	if err != nil {
		return Task{}, err
	}

	// result[0] is the key name, result[1] is the value
	var taskRef TaskReference
	if err := json.Unmarshal([]byte(result[1]), &taskRef); err != nil {
		return Task{}, err
	}

	// Get the task function from memory
	r.mu.RLock()
	payload, exists := r.taskFuncs[taskRef.ID]
	r.mu.RUnlock()

	if !exists {
		return Task{}, errors.New("task function not found")
	}

	// Create the task with the function
	task := Task{
		ID:      taskRef.ID,
		Payload: payload,
	}

	return task, nil
}

// Close closes the Redis connection
func (r *RedisTaskStorage) Close() error {
	// properly close redis connection to prevent resource leaks
	// important for clean shutdown of the queue
	// without this, redis connections might remain open
	return r.client.Close()
}
