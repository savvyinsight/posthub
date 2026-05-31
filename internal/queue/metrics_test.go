package queue

import (
	"testing"
	"time"
)

func TestNopMetrics_DoNotPanic(t *testing.T) {
	var m NopMetrics

	// All methods should be no-ops and not panic.
	m.TaskEnqueued("publish")
	m.TaskStarted("task-1", "twitter")
	m.TaskCompleted("task-1", "twitter", 500*time.Millisecond)
	m.TaskFailed("task-1", "twitter", nil)
	m.TaskRetried("task-1", "twitter", 2)
	m.TaskDead("task-1", "twitter")
	m.IdempotencyHit("content-1:twitter")
}
