# Database Schema

## Goal

Define the database schema for the content distribution system.

This document is the source of truth for:

- table definitions
- relationships
- indexes
- constraints

---

## Database Choice

PostgreSQL.

Reasons:

- JSON support for flexible fields
- transactional guarantees
- mature ecosystem
- pgx driver performance

---

## Schema Design Principles

### Separate Tasks from Attempts

A publish task is the intent to publish to a platform.

An attempt is one try at completing that task.

```
publish_task (1) → (many) publish_attempts
```

This matters for:

- observability
- debugging
- analytics
- auditing

---

## Tables

### content

Stores canonical content.

```sql
CREATE TABLE content (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    tags JSONB DEFAULT '[]',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_status ON content(status);
CREATE INDEX idx_content_created_at ON content(created_at);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | no | gen_random_uuid() | unique content identifier |
| title | VARCHAR(200) | no | - | post title |
| body | TEXT | no | - | post content in markdown |
| tags | JSONB | yes | [] | content tags array |
| status | VARCHAR(30) | no | draft | content lifecycle state |
| metadata | JSONB | yes | {} | flexible metadata |
| created_at | TIMESTAMPTZ | no | NOW() | creation timestamp |
| updated_at | TIMESTAMPTZ | no | NOW() | last update timestamp |

#### Status Values

- draft
- ready
- publishing
- published
- partially_published
- failed
- archived

---

### publish_tasks

Stores publish task intent.

```sql
CREATE TABLE publish_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_id UUID NOT NULL REFERENCES content(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    max_retries INT NOT NULL DEFAULT 3,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_publish_task_content_platform UNIQUE (content_id, platform)
);

CREATE INDEX idx_publish_tasks_content_id ON publish_tasks(content_id);
CREATE INDEX idx_publish_tasks_status ON publish_tasks(status);
CREATE INDEX idx_publish_tasks_platform ON publish_tasks(platform);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | no | gen_random_uuid() | unique task identifier |
| content_id | UUID | no | - | references content.id |
| platform | VARCHAR(50) | no | - | target platform name |
| status | VARCHAR(30) | no | pending | task lifecycle state |
| max_retries | INT | no | 3 | maximum retry attempts |
| error | TEXT | yes | null | last error message |
| created_at | TIMESTAMPTZ | no | NOW() | creation timestamp |
| updated_at | TIMESTAMPTZ | no | NOW() | last update timestamp |

#### Status Values

- pending
- processing
- succeeded
- failed
- dead
- cancelled

#### Unique Constraint

One task per content-platform pair.

Prevents duplicate publish intent.

---

### publish_attempts

Stores individual publish attempts.

```sql
CREATE TABLE publish_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'started',
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    CONSTRAINT uq_publish_attempt_number UNIQUE (task_id, attempt_number)
);

CREATE INDEX idx_publish_attempts_task_id ON publish_attempts(task_id);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | no | gen_random_uuid() | unique attempt identifier |
| task_id | UUID | no | - | references publish_tasks.id |
| attempt_number | INT | no | - | sequential attempt number |
| status | VARCHAR(30) | no | started | attempt status |
| error | TEXT | yes | null | error message if failed |
| started_at | TIMESTAMPTZ | no | NOW() | attempt start time |
| completed_at | TIMESTAMPTZ | yes | null | attempt end time |

#### Status Values

- started
- succeeded
- failed

---

### platform_posts

Stores successful platform publications.

```sql
CREATE TABLE platform_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES publish_tasks(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    platform_post_id VARCHAR(200) NOT NULL,
    platform_url VARCHAR(500),
    response JSONB,
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_posts_task_id ON platform_posts(task_id);
CREATE INDEX idx_platform_posts_platform ON platform_posts(platform);
CREATE INDEX idx_platform_posts_platform_post_id ON platform_posts(platform, platform_post_id);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | no | gen_random_uuid() | unique record identifier |
| task_id | UUID | no | - | references publish_tasks.id |
| platform | VARCHAR(50) | no | - | platform name |
| platform_post_id | VARCHAR(200) | no | - | ID assigned by platform |
| platform_url | VARCHAR(500) | yes | null | URL of published post |
| response | JSONB | yes | null | raw API response |
| published_at | TIMESTAMPTZ | no | - | platform publish timestamp |
| created_at | TIMESTAMPTZ | no | NOW() | record creation timestamp |

---

## Relationships

```
content (1) → (many) publish_tasks
publish_tasks (1) → (many) publish_attempts
publish_tasks (1) → (1) platform_posts
```

### Foreign Keys

- publish_tasks.content_id → content.id (CASCADE)
- publish_attempts.task_id → publish_tasks.id (CASCADE)
- platform_posts.task_id → publish_tasks.id (CASCADE)

### Cascade Rules

- deleting content cascades to tasks, attempts, posts
- deleting task cascades to attempts, posts

---

## UUID Generation

UUIDs generated by PostgreSQL using `gen_random_uuid()`.

Format: UUID v4

Example: `550e8400-e29b-41d4-a716-446655440000`

---

## Timestamps

All timestamps are TIMESTAMPTZ (timezone-aware).

PostgreSQL stores in UTC.

Application handles timezone conversion.

---

## JSON Fields

### content.tags

```json
["golang", "backend", "api"]
```

- type: array of strings
- max items: 10
- max item length: 50 characters

### content.metadata

```json
{
  "source": "web",
  "word_count": 1500
}
```

- type: object
- flexible schema
- for application-specific data

### platform_posts.response

```json
{
  "id": "12345",
  "url": "https://platform.com/post/12345",
  "status": "published"
}
```

- type: object
- stores raw API response
- for debugging

---

## Indexes Strategy

### Primary Indexes

- content.id
- publish_tasks.id
- publish_attempts.id
- platform_posts.id

### Secondary Indexes

- content.status: filter by status
- content.created_at: sort by date
- publish_tasks.content_id: find tasks for content
- publish_tasks.status: filter by status
- publish_tasks.platform: filter by platform
- publish_attempts.task_id: find attempts for task
- platform_posts.task_id: find post for task
- platform_posts.platform: filter by platform
- platform_posts.platform_post_id: lookup by platform ID

---

## Constraints

### Unique Constraints

- content.id: primary key
- publish_tasks.id: primary key
- publish_tasks(content_id, platform): one task per content-platform pair
- publish_attempts.id: primary key
- publish_attempts(task_id, attempt_number): sequential attempts
- platform_posts.id: primary key

### Foreign Key Constraints

- publish_tasks.content_id references content.id
- publish_attempts.task_id references publish_tasks.id
- platform_posts.task_id references publish_tasks.id

### Check Constraints

- content.status IN ('draft', 'ready', 'publishing', 'published', 'partially_published', 'failed', 'archived')
- publish_tasks.status IN ('pending', 'processing', 'succeeded', 'failed', 'dead', 'cancelled')
- publish_attempts.status IN ('started', 'succeeded', 'failed')
- publish_tasks.max_retries >= 0
- publish_attempts.attempt_number > 0

---

## Migration Strategy

Use golang-migrate for schema migrations.

Migration files:

```
migrations/
    001_create_content.up.sql
    001_create_content.down.sql
    002_create_publish_tasks.up.sql
    002_create_publish_tasks.down.sql
    003_create_publish_attempts.up.sql
    003_create_publish_attempts.down.sql
    004_create_platform_posts.up.sql
    004_create_platform_posts.down.sql
```

---

## Query Examples

### sqlc Query Format

```sql
-- name: CreateContent :one
INSERT INTO content (title, body, tags)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetContent :one
SELECT * FROM content WHERE id = $1;

-- name: ListContentByStatus :many
SELECT * FROM content
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateContentStatus :exec
UPDATE content
SET status = $2, updated_at = NOW()
WHERE id = $1;

-- name: CreatePublishTask :one
INSERT INTO publish_tasks (content_id, platform)
VALUES ($1, $2)
RETURNING *;

-- name: GetPendingTasks :many
SELECT * FROM publish_tasks
WHERE status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: UpdateTaskStatus :exec
UPDATE publish_tasks
SET status = $2, error = $3, updated_at = NOW()
WHERE id = $1;

-- name: CreatePublishAttempt :one
INSERT INTO publish_attempts (task_id, attempt_number)
VALUES ($1, $2)
RETURNING *;

-- name: CompleteAttempt :exec
UPDATE publish_attempts
SET status = $2, error = $3, completed_at = NOW()
WHERE id = $1;

-- name: CreatePlatformPost :one
INSERT INTO platform_posts (task_id, platform, platform_post_id, platform_url, response, published_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
```

---

## Non-Goals For MVP

Not included:

- soft deletes
- audit logging tables
- user tables
- platform account tables
- analytics tables
- asset storage tables
