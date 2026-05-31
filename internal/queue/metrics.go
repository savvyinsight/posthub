// Metrics provides hooks for queue observability.
//
// Implementations are injected; the default is NopMetrics.
// All methods are called at the corresponding lifecycle point
// in the task handler and enqueuer.
package queue

import "time"

// Metrics exposes queue lifecycle events for observability.
//
// Implementations may emit Prometheus counters, StatsD gauges,
// or any other metrics backend. NopMetrics is the default.
type Metrics interface {
	// TaskEnqueued is called when a task is successfully enqueued.
	TaskEnqueued(queue string)
	// TaskStarted is called when a handler begins processing a task.
	TaskStarted(taskID, platform string)
	// TaskCompleted is called when a task succeeds.
	TaskCompleted(taskID, platform string, duration time.Duration)
	// TaskFailed is called when a task fails (retryable or not).
	TaskFailed(taskID, platform string, err error)
	// TaskRetried is called when a retryable task is rescheduled.
	TaskRetried(taskID, platform string, attempt int)
	// TaskDead is called when a task is moved to the dead-letter queue.
	TaskDead(taskID, platform string)
	// IdempotencyHit is called when a duplicate task is detected and skipped.
	IdempotencyHit(key string)
}

// NopMetrics is a no-op implementation of Metrics.
// Used when no metrics backend is configured.
type NopMetrics struct{}

func (NopMetrics) TaskEnqueued(string)                        {}
func (NopMetrics) TaskStarted(string, string)                 {}
func (NopMetrics) TaskCompleted(string, string, time.Duration) {}
func (NopMetrics) TaskFailed(string, string, error)           {}
func (NopMetrics) TaskRetried(string, string, int)            {}
func (NopMetrics) TaskDead(string, string)                    {}
func (NopMetrics) IdempotencyHit(string)                      {}
