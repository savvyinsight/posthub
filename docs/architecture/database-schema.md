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
- Go driver support

---

## Tables

### content

Stores canonical content.

```sql
CREATE TABLE content (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    tags JSONB DEFAULT '[]',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_content_status ON content(status);
CREATE INDEX idx_content_created_at ON content(created_at);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(36) | no | uuid | unique content identifier |
| title | VARCHAR(200) | no | - | post title |
| body | TEXT | no | - | post content in markdown |
| tags | JSONB | yes | [] | content tags array |
| status | VARCHAR(20) | no | draft | content lifecycle state |
| created_at | TIMESTAMP | no | now() | creation timestamp |
| updated_at | TIMESTAMP | no | now() | last update timestamp |

#### Status Values

- draft
- ready
- publishing
- published
- archived

---

### publish_jobs

Stores publish job state.

```sql
CREATE TABLE publish_jobs (
    id VARCHAR(36) PRIMARY KEY,
    content_id VARCHAR(36) NOT NULL REFERENCES content(id),
    platform VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    retry_count INT NOT NULL DEFAULT 0,
    max_retries INT NOT NULL DEFAULT 3,
    error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_publish_jobs_content_id ON publish_jobs(content_id);
CREATE INDEX idx_publish_jobs_status ON publish_jobs(status);
CREATE INDEX idx_publish_jobs_platform ON publish_jobs(platform);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(36) | no | uuid | unique job identifier |
| content_id | VARCHAR(36) | no | - | references content.id |
| platform | VARCHAR(50) | no | - | target platform name |
| status | VARCHAR(20) | no | queued | job lifecycle state |
| retry_count | INT | no | 0 | current retry attempt |
| max_retries | INT | no | 3 | maximum retry attempts |
| error | TEXT | yes | null | last error message |
| created_at | TIMESTAMP | no | now() | creation timestamp |
| updated_at | TIMESTAMP | no | now() | last update timestamp |

#### Status Values

- queued
- processing
- success
- failed
- retrying
- dead

---

### publish_results

Stores platform publish results.

```sql
CREATE TABLE publish_results (
    id VARCHAR(36) PRIMARY KEY,
    job_id VARCHAR(36) NOT NULL REFERENCES publish_jobs(id),
    platform_post_id VARCHAR(200),
    response JSONB,
    published_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_publish_results_job_id ON publish_results(job_id);
```

#### Column Definitions

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | VARCHAR(36) | no | uuid | unique result identifier |
| job_id | VARCHAR(36) | no | - | references publish_jobs.id |
| platform_post_id | VARCHAR(200) | yes | null | ID assigned by platform |
| response | JSONB | yes | null | raw API response |
| published_at | TIMESTAMP | yes | null | platform publish timestamp |
| created_at | TIMESTAMP | no | now() | record creation timestamp |

---

## Relationships

```
content (1) → (many) publish_jobs
publish_jobs (1) → (1) publish_results
```

### Foreign Keys

- publish_jobs.content_id → content.id
- publish_results.job_id → publish_jobs.id

### Cascade Rules

- deleting content cascades to publish_jobs
- deleting publish_jobs cascades to publish_results

---

## UUID Generation

UUIDs are generated in application code.

Format: UUID v4

Example: `550e8400-e29b-41d4-a716-446655440000`

---

## Timestamps

All timestamps are UTC.

Stored as TIMESTAMP WITHOUT TIME ZONE.

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

### publish_results.response

```json
{
  "id": "12345",
  "url": "https://platform.com/post/12345",
  "status": "published"
}
```

- type: object
- stores raw API response
- for debugging only

---

## Indexes Strategy

### Primary Indexes

- content.id
- publish_jobs.id
- publish_results.id

### Secondary Indexes

- content.status: filter by status
- content.created_at: sort by date
- publish_jobs.content_id: find jobs for content
- publish_jobs.status: filter by status
- publish_jobs.platform: filter by platform
- publish_results.job_id: find result for job

---

## Constraints

### Unique Constraints

- content.id: primary key
- publish_jobs.id: primary key
- publish_results.id: primary key

### Foreign Key Constraints

- publish_jobs.content_id references content.id
- publish_results.job_id references publish_jobs.id

### Check Constraints

- content.status IN ('draft', 'ready', 'publishing', 'published', 'archived')
- publish_jobs.status IN ('queued', 'processing', 'success', 'failed', 'retrying', 'dead')
- publish_jobs.retry_count >= 0
- publish_jobs.max_retries >= 0

---

## Migration Strategy

Use golang-migrate for schema migrations.

Migration files:

```
migrations/
    001_create_content.up.sql
    001_create_content.down.sql
    002_create_publish_jobs.up.sql
    002_create_publish_jobs.down.sql
    003_create_publish_results.up.sql
    003_create_publish_results.down.sql
```

---

## Non-Goals For MVP

Not included:

- soft deletes
- audit logging tables
- user tables
- platform account tables
- analytics tables
