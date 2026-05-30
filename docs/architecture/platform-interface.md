# Platform Interface

## Goal

Define the interface contract for platform publishers.

This document is the source of truth for:

- publisher interface
- platform abstraction
- authentication handling
- error classification

---

## Publisher Interface

Every platform publisher must implement this interface:

```go
type Publisher interface {
    // Name returns the platform identifier
    Name() string

    // Publish sends content to the platform
    Publish(ctx context.Context, content *CanonicalContent) (*PublishResult, error)

    // Validate checks if content meets platform requirements
    Validate(content *CanonicalContent) error

    // Authenticate sets up authentication for the platform
    Authenticate(ctx context.Context) error
}
```

---

## Platform Registration

Platforms are registered at startup:

```go
registry := NewPublisherRegistry()
registry.Register(zhihu.NewPublisher(config))
registry.Register(bilibili.NewPublisher(config))
```

Workers fetch publishers from registry by name.

---

## Canonical Content Input

Publishers receive canonical content:

```go
type CanonicalContent struct {
    ID     string
    Title  string
    Body   string
    Tags   []string
}
```

Publishers must NOT modify canonical content.

---

## Publish Result

Publishers return:

```go
type PublishResult struct {
    PlatformPostID string
    PublishedAt    time.Time
    Response       json.RawMessage
}
```

| Field | Description |
|-------|-------------|
| PlatformPostID | ID assigned by the platform |
| PublishedAt | timestamp of publication |
| Response | raw API response for debugging |

---

## Error Classification

Publishers must classify errors:

### Retryable Errors

```go
type RetryableError struct {
    Message string
    RetryAfter time.Duration
}
```

Examples:

- network timeout
- rate limit exceeded
- server error (5xx)

### Permanent Errors

```go
type PermanentError struct {
    Message string
    Code    string
}
```

Examples:

- invalid content
- authentication failure
- API rejection

### Unknown Errors

All other errors are treated as retryable.

---

## Platform Validation

Each platform defines its own validation rules:

### Zhihu

- title: max 100 characters
- body: max 100000 characters
- tags: max 5 tags

### Bilibili

- title: max 80 characters
- body: max 50000 characters
- tags: max 10 tags

Validation runs before publishing.

---

## Authentication

### Authentication Flow

```
Publisher.Authenticate(ctx)
    ↓
Load credentials from config
    ↓
Validate credentials
    ↓
Cache token if applicable
    ↓
Return error if failed
```

### Credential Storage

Credentials are stored in:

- environment variables
- or config file

Never in database.

### Token Refresh

Publishers handle token refresh internally.

Worker does not manage tokens.

---

## Platform Configuration

Each platform requires:

```go
type PlatformConfig struct {
    Name        string
    Enabled     bool
    Credentials map[string]string
    Options     map[string]string
}
```

Example:

```yaml
platforms:
  zhihu:
    enabled: true
    credentials:
      client_id: xxx
      client_secret: xxx
  bilibili:
    enabled: true
    credentials:
      cookie: xxx
```

---

## Content Transformation

Transformation happens before validation:

```
CanonicalContent
    ↓
Transform for platform
    ↓
PlatformContent
    ↓
Validate
    ↓
Publish
```

Transformation is deterministic.

No AI in MVP.

---

## Adding New Platforms

To add a new platform:

1. implement Publisher interface
2. define validation rules
3. register at startup
4. add configuration

No changes to core system required.

---

## Non-Goals For MVP

Not included:

- platform webhooks
- platform event streaming
- multi-account per platform
- platform-specific analytics
- platform rate limiting
