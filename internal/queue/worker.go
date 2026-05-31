// Worker bootstraps and runs the Asynq task processing server.
//
// Also provides DLQInspector for dead-letter queue management.
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
	"github.com/savvyinsight/posthub/internal/logger"
	"go.uber.org/zap"
)

// WorkerConfig configures the Asynq worker server.
type WorkerConfig struct {
	// RedisAddr is the Redis address (host:port).
	RedisAddr string
	// RedisPassword for authentication (empty = no auth).
	RedisPassword string
	// RedisDB selects the Redis database (0-15).
	RedisDB int
	// Concurrency is the max number of concurrent task handlers.
	Concurrency int
	// Queues maps queue names to priority weights.
	// Higher weight = higher priority. Example: {"publish": 6, "default": 4}.
	Queues map[string]int
}

// DefaultWorkerConfig returns a sensible default configuration.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		RedisAddr:   "localhost:6379",
		Concurrency: 10,
		Queues: map[string]int{
			"publish": 6,
			"default": 4,
		},
	}
}

// Worker wraps an Asynq server with lifecycle management.
type Worker struct {
	srv       *asynq.Server
	mux       *asynq.ServeMux
	inspector *asynq.Inspector
	log       *logger.Logger
}

// NewWorker creates a Worker that processes tasks via the given handler.
//
// Task type registration happens inside this constructor — the caller
// only needs to provide the Handler implementation.
func NewWorker(cfg WorkerConfig, handler Handler, log *logger.Logger) *Worker {
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Concurrency,
		Queues:      cfg.Queues,
	})

	mux := asynq.NewServeMux()
	mux.HandleFunc(TypePublishContent, func(ctx context.Context, t *asynq.Task) error {
		var payload PublishPayload
		if err := decodePayload(t.Payload(), &payload); err != nil {
			log.Error("failed to decode task payload",
				zap.String("type", t.Type()),
				zap.Error(err),
			)
			return fmt.Errorf("decode payload: %w", err)
		}

		return handler.HandlePublish(ctx, payload)
	})

	inspector := asynq.NewInspector(redisOpt)

	return &Worker{
		srv:       srv,
		mux:       mux,
		inspector: inspector,
		log:       log,
	}
}

// Run starts the worker and blocks until it shuts down.
// It returns when the context is cancelled or the server encounters
// a fatal error.
func (w *Worker) Run(ctx context.Context) error {
	w.log.Info("starting worker")

	// Run blocks until Shutdown is called or an error occurs.
	if err := w.srv.Run(w.mux); err != nil {
		w.log.Error("worker stopped with error", zap.Error(err))
		return fmt.Errorf("worker run: %w", err)
	}

	w.log.Info("worker stopped")
	return nil
}

// Shutdown gracefully stops the worker, waiting for active tasks to finish.
func (w *Worker) Shutdown() {
	w.log.Info("shutting down worker")
	w.srv.Shutdown()
}

// DLQ returns a DLQInspector for dead-letter queue management.
func (w *Worker) DLQ() *DLQInspector {
	return &DLQInspector{
		inspector: w.inspector,
		log:       w.log,
	}
}

// DLQInspector provides operations on the dead-letter (archived) queue.
//
// In Asynq, tasks that exhaust all retries are moved to the "archived"
// state. This inspector wraps Asynq's Inspector for DLQ management.
type DLQInspector struct {
	inspector *asynq.Inspector
	log       *logger.Logger
}

// DeadTaskCount returns the number of archived (dead) tasks in the given queue.
func (d *DLQInspector) DeadTaskCount(ctx context.Context, queue string) (int, error) {
	tasks, err := d.inspector.ListArchivedTasks(queue)
	if err != nil {
		return 0, fmt.Errorf("list archived tasks: %w", err)
	}
	return len(tasks), nil
}

// ListDeadTasks returns all archived (dead) tasks in the given queue.
func (d *DLQInspector) ListDeadTasks(ctx context.Context, queue string) ([]*asynq.TaskInfo, error) {
	tasks, err := d.inspector.ListArchivedTasks(queue)
	if err != nil {
		return nil, fmt.Errorf("list archived tasks: %w", err)
	}
	d.log.Info("listed archived tasks",
		zap.String("queue", queue),
		zap.Int("count", len(tasks)),
	)
	return tasks, nil
}

// RetryDeadTask re-enqueues a single archived task for processing.
func (d *DLQInspector) RetryDeadTask(ctx context.Context, queue, asynqID string) error {
	if err := d.inspector.RunTask(queue, asynqID); err != nil {
		d.log.Error("failed to retry archived task",
			zap.String("queue", queue),
			zap.String("asynq_id", asynqID),
			zap.Error(err),
		)
		return fmt.Errorf("retry archived task: %w", err)
	}
	d.log.Info("retried archived task",
		zap.String("queue", queue),
		zap.String("asynq_id", asynqID),
	)
	return nil
}

// RetryAllDeadTasks re-enqueues every archived task in the given queue.
func (d *DLQInspector) RetryAllDeadTasks(ctx context.Context, queue string) (int, error) {
	n, err := d.inspector.RunAllArchivedTasks(queue)
	if err != nil {
		d.log.Error("failed to retry all archived tasks",
			zap.String("queue", queue),
			zap.Error(err),
		)
		return 0, fmt.Errorf("retry all archived tasks: %w", err)
	}
	d.log.Info("retried all archived tasks",
		zap.String("queue", queue),
		zap.Int("count", n),
	)
	return n, nil
}

// DeleteDeadTask removes a single archived task from the queue.
func (d *DLQInspector) DeleteDeadTask(ctx context.Context, queue, asynqID string) error {
	if err := d.inspector.DeleteTask(queue, asynqID); err != nil {
		d.log.Error("failed to delete archived task",
			zap.String("queue", queue),
			zap.String("asynq_id", asynqID),
			zap.Error(err),
		)
		return fmt.Errorf("delete archived task: %w", err)
	}
	d.log.Info("deleted archived task",
		zap.String("queue", queue),
		zap.String("asynq_id", asynqID),
	)
	return nil
}

// DeleteAllDeadTasks removes all archived tasks from the given queue.
func (d *DLQInspector) DeleteAllDeadTasks(ctx context.Context, queue string) (int, error) {
	n, err := d.inspector.DeleteAllArchivedTasks(queue)
	if err != nil {
		d.log.Error("failed to delete all archived tasks",
			zap.String("queue", queue),
			zap.Error(err),
		)
		return 0, fmt.Errorf("delete all archived tasks: %w", err)
	}
	d.log.Info("deleted all archived tasks",
		zap.String("queue", queue),
		zap.Int("count", n),
	)
	return n, nil
}

// decodePayload unmarshals raw task JSON into the target struct.
func decodePayload(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
