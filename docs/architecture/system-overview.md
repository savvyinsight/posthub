# System Overview

## Goal

Provide a high-level overview of the content distribution system.

This document is the source of truth for:

- system purpose
- architecture overview
- component responsibilities
- technology choices

---

## What Is PostHub

PostHub is a content distribution system.

It allows users to:

- create content once
- publish to multiple platforms
- track publish status

---

## Core Systems

PostHub consists of three core systems:

```
Canonical Content System
        +
Publish Orchestration System
        +
Platform Adapter System
```

This is NOT merely:

```
API + Queue + Worker
```

---

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────────────────────────────────────────┐
│                  API Server                      │
│         (chi router, slog logging)               │
└──────┬──────────────────────────────────┬───────┘
       │                                  │
       ▼                                  ▼
┌─────────────────┐              ┌─────────────────┐
│  Content Domain │              │ Publish Domain  │
│  (CRUD, state)  │              │ (orchestration) │
└────────┬────────┘              └────────┬────────┘
         │                                │
         ▼                                ▼
┌─────────────────┐              ┌─────────────────┐
│    PostgreSQL   │              │  Asynq (Redis)  │
│   (pgx + sqlc)  │              │  (task queue)   │
└─────────────────┘              └────────┬────────┘
                                          │
                                          ▼
                                 ┌─────────────────┐
                                 │  Worker Pool    │
                                 │  (Asynq worker) │
                                 └────────┬────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    ▼                     ▼                     ▼
           ┌─────────────┐      ┌─────────────┐      ┌─────────────┐
           │  Zhihu      │      │  Bilibili   │      │   Weibo     │
           │  Adapter    │      │  Adapter    │      │  Adapter    │
           └─────────────┘      └─────────────┘      └─────────────┘
```

---

## Domain Boundaries

### Content Domain

Responsibilities:

- content CRUD operations
- content state management
- content validation

Location: `internal/domain/content/`

---

### Publishing Domain

Responsibilities:

- publish orchestration
- task management
- state machine execution
- result tracking

Location: `internal/domain/publishing/`

---

### Platform Adapters

Responsibilities:

- platform authentication
- content transformation
- asset upload
- API communication

Location: `internal/platforms/`

---

### Workflow Engine

Responsibilities:

- task scheduling
- retry logic
- error classification
- dead letter handling

Location: `internal/workflow/`

---

### Storage Layer

Responsibilities:

- database access
- query execution
- transaction management

Location: `internal/storage/`

---

### Queue Layer

Responsibilities:

- task enqueue
- task processing
- Asynq integration

Location: `internal/queue/`

---

### API Layer

Responsibilities:

- HTTP routing
- request parsing
- response formatting
- middleware

Location: `internal/api/`

---

## Components

### API Server

Responsibilities:

- accept HTTP requests
- validate input
- manage content lifecycle
- enqueue publish tasks
- return responses

Technology: chi router

---

### PostgreSQL

Responsibilities:

- store canonical content
- store publish tasks
- store publish attempts
- store platform posts

Why PostgreSQL:

- JSON support for metadata
- transactional guarantees
- mature ecosystem

Technology: pgx driver + sqlc

---

### Redis + Asynq

Responsibilities:

- task queue (via Asynq)
- retry management (built-in)
- dead letter queue (built-in)
- scheduling (future)

Why Asynq:

- Go-native
- Redis-based
- production-tested
- retries built-in
- DLQ built-in
- no custom queue code needed

---

### Workers

Responsibilities:

- process tasks from Asynq queue
- load content from database
- transform content via pipeline
- publish via platform adapter
- store results
- handle retries (via Asynq)

Technology: Asynq worker

---

### Platform Adapters

Responsibilities:

- authenticate with platform
- transform content via IR
- upload assets
- call platform API
- parse response

Each platform is a separate adapter implementing the Publisher interface.

---

### Transformation Pipeline

Responsibilities:

- parse markdown to AST
- convert AST to IR (Intermediate Representation)
- render IR to platform-specific payload

Technology: goldmark parser

---

## Data Flow

### Content Creation

```
Client → API → Content Domain → PostgreSQL
```

### Publishing

```
Client → API → Publish Domain → Asynq → Worker → Asset Pipeline → Platform Adapter → Platform
                                                → PostgreSQL (store result)
```

### Status Check

```
Client → API → Publish Domain → PostgreSQL
```

---

## Technology Stack

| Component | Technology | Why |
|-----------|------------|-----|
| Language | Go | performance, simplicity |
| HTTP Router | chi | lightweight, idiomatic |
| Database | PostgreSQL | reliability, JSON support |
| DB Driver | pgx | performance, features |
| DB Queries | sqlc | type-safe, no ORM |
| Queue | Asynq | production-tested, built-in retries |
| Migrations | golang-migrate | simple, versioned |
| Logging | slog | structured, stdlib |
| Config | env vars | 12-factor app |
| Markdown | goldmark | extensible, CommonMark |

---

## Project Structure

```
posthub/
    cmd/
        api/                    # API server entry point
        worker/                 # worker entry point
    internal/
        domain/
            content/            # content aggregate
            publishing/         # publish orchestration
        platforms/
            zhihu/              # Zhihu adapter
            bilibili/           # Bilibili adapter
            weibo/              # Weibo adapter
        workflow/               # state machine, retry logic
        storage/                # database access layer
        queue/                  # Asynq integration
        api/                    # HTTP handlers, routes
        transform/              # IR, parser, renderers
    migrations/                 # database migrations
    docs/                       # documentation
    config/                     # configuration
```

---

## Configuration

Configuration via environment variables:

```env
DATABASE_URL=postgres://localhost:5432/posthub
REDIS_URL=redis://localhost:6379
API_PORT=8080
WORKER_CONCURRENCY=5
LOG_LEVEL=info
```

---

## Deployment

### Development

```bash
# Start dependencies
docker-compose up -d

# Run migrations
make migrate-up

# Start API
go run cmd/api/main.go

# Start worker
go run cmd/worker/main.go
```

### Production

Two binaries: api and worker.

Build:

```bash
go build -o posthub-api ./cmd/api
go build -o posthub-worker ./cmd/worker
```

---

## Observability

### Logging

Structured logging with slog.

Log levels:

- debug
- info
- warn
- error

### Health Check

GET /health

Returns:

- service status
- dependency status
- version

---

## Security

### MVP

- no authentication
- no authorization
- no rate limiting

### Future

- API key authentication
- rate limiting
- input sanitization
- CORS configuration

---

## Non-Goals For MVP

Not included:

- user management
- multi-tenancy
- analytics
- webhooks
- scheduling
- AI transformation
- permanent asset storage

---

## Related Documents

- [publish-workflow.md](publish-workflow.md)
- [canonical-content.md](canonical-content.md)
- [platform-interface.md](platform-interface.md)
- [queue-workflow.md](queue-workflow.md)
- [database-schema.md](database-schema.md)
- [api-design.md](api-design.md)
- [error-handling.md](error-handling.md)
- [transformation-pipeline.md](transformation-pipeline.md)
- [publish-state-machine.md](publish-state-machine.md)
- [asset-pipeline.md](asset-pipeline.md)
- [platform-capability-matrix.md](platform-capability-matrix.md)
- [auth-provider-architecture.md](auth-provider-architecture.md)
- [rate-limit-strategy.md](rate-limit-strategy.md)
- [platform-abstraction.md](platform-abstraction.md)
