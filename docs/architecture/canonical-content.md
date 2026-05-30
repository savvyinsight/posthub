# Canonical Content

## Goal

Define the core content model that serves as the single source of truth for all platform publishing.

This document is the source of truth for:

- content structure
- content storage
- content lifecycle
- content validation

---

## What Is Canonical Content

Canonical content is the platform-independent representation of a post.

It is:

- stored once in the database
- transformed per platform before publishing
- never modified after creation

It is NOT:

- platform-specific formatting
- platform-specific metadata
- published post references

---

## Content Model

### Core Fields

```json
{
  "id": "content_xxx",
  "title": "Example Post",
  "body": "Main content in markdown",
  "tags": ["golang", "backend"],
  "status": "draft",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z"
}
```

### Field Definitions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| id | string | yes | unique content identifier |
| title | string | yes | post title |
| body | string | yes | post content in markdown |
| tags | string[] | no | content tags |
| status | enum | yes | content lifecycle state |
| created_at | timestamp | yes | creation time |
| updated_at | timestamp | yes | last update time |

---

## Content Status

```
draft
    ↓
ready
    ↓
publishing
    ↓
published
    ↓
archived
```

### Status Definitions

- **draft**: content is being edited
- **ready**: content is finalized and can be published
- **publishing**: content is being published to one or more platforms
- **published**: content has been published to at least one platform
- **archived**: content is no longer active

---

## Content Validation

### Creation Validation

- title: required, max 200 characters
- body: required, min 1 character
- tags: optional, max 10 tags, each max 50 characters

### Update Validation

- only draft content can be updated
- published content cannot be modified
- archived content cannot be modified

---

## Content Lifecycle

### Create

```
Client → POST /content → Store → Return content_id
```

- always created as draft
- returns content ID
- no publishing occurs

### Update

```
Client → PUT /content/:id → Validate status → Update → Return
```

- only draft content can be updated
- validates all fields
- updates updated_at timestamp

### Publish

```
Client → POST /content/:id/publish → Validate status → Create jobs → Queue
```

- only ready content can be published
- creates publish jobs per platform
- updates status to publishing

### Archive

```
Client → POST /content/:id/archive → Validate status → Update status
```

- only published content can be archived
- does not unpublish from platforms
- marks content as archived

---

## Content Storage

### Database Table

```sql
CREATE TABLE content (
    id VARCHAR(36) PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    tags JSON,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

### Indexes

- primary: id
- index: status
- index: created_at

---

## Content Immutability

Once content is published:

- title cannot be changed
- body cannot be changed
- tags cannot be changed

This ensures:

- published posts remain consistent
- re-publishing produces same result
- audit trail is accurate

---

## Content And Publishing Relationship

```
content (1) → (many) publish_jobs
```

One content can have multiple publish jobs.

Each publish job:

- references one content
- targets one platform
- has independent status

---

## Non-Goals For MVP

Not included:

- content versioning
- content templates
- rich text editor support
- image attachments
- content scheduling
- content collaboration
