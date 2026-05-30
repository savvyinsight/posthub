# API Design

## Goal

Define the REST API for the content distribution system.

This document is the source of truth for:

- endpoint definitions
- request/response formats
- error handling
- authentication

---

## Base URL

```
http://localhost:8080/api/v1
```

Versioning: URL path prefix.

---

## Content Endpoints

### Create Content

```
POST /content
```

Request:

```json
{
  "title": "Example Post",
  "body": "Main content",
  "tags": ["golang", "backend"]
}
```

Response (201):

```json
{
  "id": "content_xxx",
  "title": "Example Post",
  "body": "Main content",
  "tags": ["golang", "backend"],
  "status": "draft",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

Validation Errors (400):

```json
{
  "error": "validation_error",
  "message": "title is required",
  "details": [
    {
      "field": "title",
      "message": "title is required"
    }
  ]
}
```

---

### List Content

```
GET /content
```

Query Parameters:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| status | string | all | filter by status |
| page | int | 1 | page number |
| per_page | int | 20 | items per page |

Response (200):

```json
{
  "data": [
    {
      "id": "content_xxx",
      "title": "Example Post",
      "status": "draft",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}
```

---

### Get Content

```
GET /content/:id
```

Response (200):

```json
{
  "id": "content_xxx",
  "title": "Example Post",
  "body": "Main content",
  "tags": ["golang", "backend"],
  "status": "draft",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

Not Found (404):

```json
{
  "error": "not_found",
  "message": "content not found"
}
```

---

### Update Content

```
PUT /content/:id
```

Request:

```json
{
  "title": "Updated Title",
  "body": "Updated content",
  "tags": ["golang", "api"]
}
```

Response (200):

```json
{
  "id": "content_xxx",
  "title": "Updated Title",
  "body": "Updated content",
  "tags": ["golang", "api"],
  "status": "draft",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

Forbidden (403):

```json
{
  "error": "forbidden",
  "message": "only draft content can be updated"
}
```

---

### Delete Content

```
DELETE /content/:id
```

Response (204): No body.

Forbidden (403):

```json
{
  "error": "forbidden",
  "message": "only draft content can be deleted"
}
```

---

## Publishing Endpoints

### Publish Content

```
POST /content/:id/publish
```

Request:

```json
{
  "platforms": ["zhihu", "bilibili"]
}
```

Response (202):

```json
{
  "content_id": "content_xxx",
  "jobs": [
    {
      "id": "job_xxx",
      "platform": "zhihu",
      "status": "queued"
    },
    {
      "id": "job_yyy",
      "platform": "bilibili",
      "status": "queued"
    }
  ]
}
```

Validation Error (400):

```json
{
  "error": "validation_error",
  "message": "content not ready for publishing"
}
```

---

### Get Publish Jobs

```
GET /content/:id/jobs
```

Response (200):

```json
{
  "data": [
    {
      "id": "job_xxx",
      "platform": "zhihu",
      "status": "success",
      "retry_count": 0,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

---

### Get Job Status

```
GET /jobs/:id
```

Response (200):

```json
{
  "id": "job_xxx",
  "content_id": "content_xxx",
  "platform": "zhihu",
  "status": "success",
  "retry_count": 0,
  "error": null,
  "result": {
    "platform_post_id": "12345",
    "published_at": "2024-01-01T00:00:00Z"
  },
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

---

## Health Endpoint

### Health Check

```
GET /health
```

Response (200):

```json
{
  "status": "ok",
  "version": "1.0.0",
  "dependencies": {
    "database": "ok",
    "redis": "ok"
  }
}
```

Service Unavailable (503):

```json
{
  "status": "degraded",
  "version": "1.0.0",
  "dependencies": {
    "database": "ok",
    "redis": "error"
  }
}
```

---

## Error Format

All errors follow this format:

```json
{
  "error": "error_code",
  "message": "human readable message",
  "details": []
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| validation_error | 400 | request validation failed |
| not_found | 404 | resource not found |
| forbidden | 403 | operation not allowed |
| internal_error | 500 | server error |

---

## Request Validation

### Content Creation

- title: required, max 200 characters
- body: required, min 1 character
- tags: optional, max 10 items, each max 50 characters

### Content Update

- title: optional, max 200 characters
- body: optional, min 1 character
- tags: optional, max 10 items, each max 50 characters

### Publish Request

- platforms: required, array of strings, must be registered platforms

---

## Response Envelope

Single resource:

```json
{
  "id": "xxx",
  "field": "value"
}
```

Collection:

```json
{
  "data": [...],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 100
  }
}
```

---

## Content Type

Request: `application/json`

Response: `application/json`

---

## Authentication

MVP: no authentication.

Future: API key in header.

```
Authorization: Bearer <api_key>
```

---

## Rate Limiting

MVP: no rate limiting.

Future: per-client rate limits.

---

## Non-Goals For MVP

Not included:

- authentication
- rate limiting
- webhooks
- bulk operations
- file uploads
