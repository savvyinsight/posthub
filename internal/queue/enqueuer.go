// Asynq-backed Enqueuer implementation.
//
// Wraps asynq.Client to implement the Enqueuer interface, mapping
// EnqueueOptions to Asynq task options including custom retry
// backoff from contracts.RetryPolicy.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/savvyinsight/posthub/internal/logger"
	"go.uber.org/zap"
)

// AsynqEnqueuer implements Enqueuer using Asynq's Redis-backed client.
type AsynqEnqueuer struct {
	client *asynq.Client
	log    *logger.Logger
}

// Compile-time interface check.
var _ Enqueuer = (*AsynqEnqueuer)(nil)

// NewAsynqEnqueuer creates an Enqueuer backed by Asynq.
func NewAsynqEnqueuer(client *asynq.Client, log *logger.Logger) *AsynqEnqueuer {
	return &AsynqEnqueuer{
		client: client,
		log:    log,
	}
}

// EnqueuePublish serializes the payload and enqueues it via Asynq.
func (e *AsynqEnqueuer) EnqueuePublish(ctx context.Context, payload PublishPayload, opts EnqueueOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal publish payload: %w", err)
	}

	asynqOpts := e.buildOptions(opts)

	task := asynq.NewTask(TypePublishContent, data)
	info, err := e.client.EnqueueContext(ctx, task, asynqOpts...)
	if err != nil {
		e.log.Error("failed to enqueue publish task",
			zap.String("task_id", payload.TaskID),
			zap.String("content_id", payload.ContentID),
			zap.String("platform", payload.Platform),
			zap.String("queue", opts.Queue),
			zap.Error(err),
		)
		return fmt.Errorf("enqueue publish task: %w", err)
	}

	e.log.Info("enqueued publish task",
		zap.String("task_id", payload.TaskID),
		zap.String("content_id", payload.ContentID),
		zap.String("platform", payload.Platform),
		zap.String("asynq_id", info.ID),
		zap.String("queue", info.Queue),
		zap.Int("max_retry", info.MaxRetry),
	)

	return nil
}

// buildOptions maps EnqueueOptions to Asynq task options.
//
// RetryPolicy is used to derive MaxRetry. The backoff strategy itself
// is configured at the server level via WorkerConfig.
func (e *AsynqEnqueuer) buildOptions(opts EnqueueOptions) []asynq.Option {
	asynqOpts := []asynq.Option{
		asynq.Queue(opts.Queue),
		asynq.MaxRetry(opts.MaxRetry),
	}

	if opts.Timeout > 0 {
		asynqOpts = append(asynqOpts, asynq.Timeout(time.Duration(opts.Timeout)*time.Second))
	}

	return asynqOpts
}
