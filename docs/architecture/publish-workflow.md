# Publish Workflow

## Goal

Define the complete publishing lifecycle for the content distribution system.

This document is the source of truth for:

- workflow execution
- queue processing
- worker responsibilities
- publishing states
- retry behavior

---

## Workflow Overview

```
Create Content
    ↓
Store Canonical Content
    ↓
Create Publish Job
    ↓
Push Job To Queue
    ↓
Worker Consumes Job
    ↓
Transform Content For Platform
    ↓
Validate Platform Content
    ↓
Publish To Platform
    ↓
Store Publish Result
    ↓
Update Job Status
```

---

## Detailed Workflow

### 1. Create Content

Client sends content to backend API.

Example request:

```json
{
  "title": "Example Post",
  "body": "Main content",
  "tags": ["golang", "backend"]
}
```

Backend:

- validates request
- stores canonical content
- returns content ID

No publishing occurs in this step.

---

### 2. Create Publish Job

When user clicks publish:

Backend creates:

- one publish job per platform

Example:

```
content_id = xxx

jobs:
- zhihu
- bilibili
```

Each job is independent.

Failure of one platform must not affect others.

---

### 3. Push Job To Queue

Backend pushes jobs into Redis queue.

Queue payload example:

```json
{
  "job_id": "job_xxx",
  "content_id": "content_xxx",
  "platform": "zhihu"
}
```

---

### 4. Worker Consumes Job

Worker:

- fetches job
- loads canonical content from database
- starts processing

Worker must be stateless.

---

### 5. Transform Content

Worker transforms canonical content into platform-specific content.

Example:

- Zhihu:
  - structured article
  - markdown cleanup
- Bilibili:
  - shorter paragraphs
  - topic tags

Transformation is deterministic.

No AI in MVP.

---

### 6. Validate Platform Content

Before publishing:

- title length validation
- tag count validation
- required field validation

Validation failure:

- mark job failed
- persist failure reason

Do not publish invalid content.

---

### 7. Publish To Platform

Worker calls platform publisher.

Publisher responsibilities:

- authentication
- request building
- API calling
- response parsing

---

### 8. Store Publish Result

Persist:

- platform post ID
- response payload
- publish timestamp
- status

---

### 9. Update Job Status

Final states:

```
queued
processing
success
failed
retrying
dead
```

---

## Retry Rules

Retry only for:

- network failure
- timeout
- temporary platform errors

Do not retry:

- invalid content
- authentication failure
- permanent API rejection

Recommended retry count:

- max 3

Recommended retry strategy:

- exponential backoff

---

## Idempotency

Publishing must be idempotent.

System must prevent:

- duplicate publish
- duplicate retry publish

Each job must contain:

- unique job ID
- unique content-platform pair

---

## Failure Isolation

Each platform job is isolated.

Example:

```
Zhihu success
Bilibili failed
```

System behavior:

- preserve Zhihu success
- retry only Bilibili

---

## Observability Requirements

Each job must log:

- job ID
- content ID
- platform
- start time
- finish time
- retry count
- error reason

---

## Non-Goals For MVP

Not included:

- scheduling
- AI transformation
- multi-account support
- analytics
- collaboration
- draft versioning
