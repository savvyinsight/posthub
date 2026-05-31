# PostHub

Content distribution system. Create once, publish everywhere.

## What Is PostHub

PostHub allows users to:

- create content in markdown
- publish to multiple social platforms simultaneously
- track publish status per platform

## Architecture

PostHub is a modular monolith with two entry points:

- **API server** (`cmd/api`) — REST API for content management and publish orchestration
- **Worker** (`cmd/worker`) — async task processor for platform publishing

### Core Systems

```
Canonical Content System
        +
Publish Orchestration System
        +
Platform Adapter System
```

### Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| HTTP Router | chi |
| Database | PostgreSQL (pgx + sqlc) |
| Queue | Asynq (Redis) |
| Logging | slog (stdlib) |
| Config | Environment variables |

## Project Structure

```
posthub/
    cmd/
        api/                    # API server entry point
        worker/                 # Worker entry point
    internal/
        config/                 # Environment-based configuration
        contracts/              # Shared domain types
        logger/                 # Structured logging setup
        platform/               # Platform interface and registry
        publish/                # Publish orchestration types
        queue/                  # Task queue abstraction
        storage/                # Database access interfaces
        transform/              # IR (Intermediate Representation) types
    scripts/                    # Operational scripts
    docs/                       # Architecture documentation
```

## Quick Start

```bash
# Build
make build

# Run tests
make test

# Run API server
make run-api

# Run worker
make run-worker
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8080` | HTTP server port |
| `DATABASE_URL` | `postgres://localhost:5432/posthub?sslmode=disable` | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection string |
| `WORKER_CONCURRENCY` | `10` | Number of concurrent workers |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `ENVIRONMENT` | `development` | Runtime environment |

## Development

```bash
# Format, vet, and test
make check

# Run with race detector
make test-race

# Coverage report
make test-cover
```

## Status

Engineering foundation phase. Core types and interfaces are defined. Business logic, database, Redis, and platform integrations are not yet implemented.
