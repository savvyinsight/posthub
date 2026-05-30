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

## Architecture

```
┌─────────────┐
│   Client    │
└──────┬──────┘
       │
       ▼
┌─────────────┐
│  API Server │
└──────┬──────┘
       │
       ├──────────────────┐
       ▼                  ▼
┌─────────────┐    ┌─────────────┐
│  PostgreSQL │    │    Redis    │
└─────────────┘    └──────┬──────┘
                          │
                          ▼
                   ┌─────────────┐
                   │   Workers   │
                   └──────┬──────┘
                          │
       ┌──────────────────┼──────────────────┐
       ▼                  ▼                  ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Zhihu     │    │  Bilibili   │    │   Other     │
└─────────────┘    └─────────────┘    └─────────────┘
```

---

## Components

### API Server

Responsibilities:

- accept HTTP requests
- validate input
- manage content lifecycle
- enqueue publish jobs
- return responses

Technology: Go net/http

---

### PostgreSQL

Responsibilities:

- store canonical content
- store publish jobs
- store publish results

Why PostgreSQL:

- JSON support
- transactional guarantees
- mature ecosystem

---

### Redis

Responsibilities:

- job queue
- idempotency set
- caching (future)

Why Redis:

- fast message broker
- simple queue operations
- built-in TTL

---

### Workers

Responsibilities:

- consume jobs from queue
- load content from database
- transform content for platform
- publish to platform
- store results
- handle retries

Technology: Go goroutines

---

### Platform Publishers

Responsibilities:

- authenticate with platform
- validate content for platform
- call platform API
- parse response

Each platform is a separate module.

---

## Data Flow

### Content Creation

```
Client → API → PostgreSQL
```

### Publishing

```
Client → API → Redis → Worker → PostgreSQL → Platform
```

### Status Check

```
Client → API → PostgreSQL
```

---

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| HTTP | net/http |
| Database | PostgreSQL |
| Queue | Redis |
| Migrations | golang-migrate |
| Logging | slog |
| Config | env vars |

---

## Project Structure

```
posthub/
    cmd/
        api/          # API server entry point
        worker/       # worker entry point
    internal/
        api/          # HTTP handlers
        content/      # content domain
        publish/      # publish domain
        queue/        # queue operations
        platform/     # platform publishers
    migrations/       # database migrations
    docs/             # documentation
    config/           # configuration
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

Single binary deployment.

Build:

```bash
go build -o posthub ./cmd/api
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

---

## Related Documents

- [publish-workflow.md](publish-workflow.md)
- [canonical-content.md](canonical-content.md)
- [platform-interface.md](platform-interface.md)
- [queue-workflow.md](queue-workflow.md)
- [database-schema.md](database-schema.md)
- [api-design.md](api-design.md)
- [error-handling.md](error-handling.md)
