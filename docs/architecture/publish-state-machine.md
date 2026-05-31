# Publish State Machine

## Goal

Define precise state transitions for content and publish tasks.

This document is the source of truth for:

- content states
- task states
- transition rules
- partial success handling

---

## Two State Machines

Content and publish tasks have separate states.

```
Content: lifecycle of the content itself
Task: lifecycle of a single platform publish attempt
```

---

## Content States

```
draft → ready → publishing → published → archived
                ↓
            partially_published → archived
                ↓
                failed → draft
```

### State Definitions

| State | Description |
|-------|-------------|
| draft | content is being edited |
| ready | content is finalized, can be published |
| publishing | at least one publish task is in progress |
| published | all publish tasks succeeded |
| partially_published | some tasks succeeded, some failed |
| failed | all publish tasks failed |
| archived | content is no longer active |

### Valid Transitions

| From | To | Trigger |
|------|-----|---------|
| draft | ready | user finalizes content |
| ready | publishing | user initiates publish |
| publishing | published | all tasks complete successfully |
| publishing | partially_published | some tasks succeed, some fail |
| publishing | failed | all tasks fail |
| partially_published | publishing | user retries failed tasks |
| published | archived | user archives content |
| partially_published | archived | user archives content |
| failed | draft | user edits and resets |

---

## Task States

```
pending → processing → succeeded
    ↓         ↓
    ↓      failed → retrying → processing
    ↓         ↓
    ↓      dead
    ↓
cancelled
```

### State Definitions

| State | Description |
|-------|-------------|
| pending | task is queued |
| processing | worker is executing task |
| succeeded | task completed successfully |
| failed | task failed permanently |
| retrying | task will be retried |
| dead | max retries exceeded |
| cancelled | user cancelled task |

### Valid Transitions

| From | To | Trigger |
|------|-----|---------|
| pending | processing | worker picks up task |
| pending | cancelled | user cancels |
| processing | succeeded | publish succeeds |
| processing | failed | permanent failure |
| processing | retrying | retryable failure |
| retrying | processing | retry attempt starts |
| retrying | dead | max retries exceeded |
| failed | dead | move to dead letter |

---

## Content State Transitions

### draft → ready

Conditions:

- title is not empty
- body is not empty
- content passes validation

Action:

- update content.status = ready

---

### ready → publishing

Conditions:

- at least one platform selected
- content.status == ready

Action:

- create publish task per platform
- update content.status = publishing

---

### publishing → published

Conditions:

- all tasks have status == succeeded

Action:

- update content.status = published
- record published_at timestamp

---

### publishing → partially_published

Conditions:

- at least one task == succeeded
- at least one task == failed or dead

Action:

- update content.status = partially_published

---

### publishing → failed

Conditions:

- all tasks have status == failed or dead

Action:

- update content.status = failed

---

### partially_published → publishing

Conditions:

- user initiates retry for failed tasks
- new tasks created for failed platforms

Action:

- create new tasks
- update content.status = publishing

---

## Task State Transitions

### pending → processing

Conditions:

- worker available
- task picked from queue

Action:

- update task.status = processing
- record started_at timestamp

---

### processing → succeeded

Conditions:

- platform API returns success
- platform post ID received

Action:

- update task.status = succeeded
- store platform_post_id
- record completed_at timestamp

---

### processing → failed

Conditions:

- permanent error (4xx, auth failure, validation)
- max retries exceeded

Action:

- update task.status = failed
- store error message
- record completed_at timestamp

---

### processing → retrying

Conditions:

- retryable error (timeout, 5xx, rate limit)
- retry_count < max_retries

Action:

- update task.status = retrying
- increment retry_count
- calculate backoff delay
- enqueue for retry

---

### retrying → processing

Conditions:

- backoff delay elapsed

Action:

- update task.status = processing
- create new attempt record

---

### retrying → dead

Conditions:

- retry_count >= max_retries

Action:

- update task.status = dead
- store final error
- move to dead letter queue

---

## Partial Success Handling

### Scenario

```
Publish to: Zhihu, Bilibili, Weibo

Zhihu: success
Bilibili: failed
Weibo: success
```

### System Behavior

1. record Zhihu success (platform_post_id stored)
2. record Weibo success (platform_post_id stored)
3. record Bilibili failure (error stored)
4. content.status = partially_published
5. user can retry Bilibili independently

### Retry Behavior

- retry only creates task for Bilibili
- Zhihu and Weibo tasks not affected
- existing platform_post_ids preserved

---

## Idempotency

### Task Level

Each task has unique ID.

Duplicate task creation prevented by:

- content_id + platform unique constraint
- check before insert

### Attempt Level

Each attempt has unique ID.

Duplicate attempt prevented by:

- attempt_id in queue payload
- processed set in Redis

---

## State Query API

### Get Content Status

```
GET /content/:id/status
```

Response:

```json
{
  "content_id": "xxx",
  "status": "partially_published",
  "tasks": [
    {
      "platform": "zhihu",
      "status": "succeeded",
      "platform_post_id": "12345"
    },
    {
      "platform": "bilibili",
      "status": "failed",
      "error": "rate limit exceeded"
    }
  ]
}
```

---

## Observability

### State Change Logging

Every state change logs:

- entity type (content/task)
- entity ID
- old state
- new state
- trigger
- timestamp

### Metrics

- time in each state
- transition counts
- failure rates by state

---

## Non-Goals For MVP

Not included:

- state machine visualization
- state change webhooks
- custom state definitions
- state-based access control
