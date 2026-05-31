# Platform Capability Matrix

## Goal

Define the capabilities and constraints of each publishing platform.

This document is the source of truth for:

- platform features
- platform limitations
- platform-specific requirements
- transformation rules

---

## Why This Matters

Each platform is different.

Transformation must account for:

- supported content types
- required fields
- validation rules
- rate limits
- auth models

---

## Platform Overview

| Platform | Content Type | Auth Model | Rate Limit |
|----------|-------------|------------|------------|
| Zhihu | Article | OAuth2 | 100/hour |
| Bilibili | Article | Cookie | 30/hour |
| Weibo | Post | OAuth2 | 200/hour |

---

## Zhihu

### Content Type

Long-form articles.

### Supported Elements

| Element | Support | Notes |
|---------|---------|-------|
| Heading | full | h1-h6 |
| Paragraph | full | |
| Code Block | full | syntax highlighting |
| Block Quote | full | |
| List | full | ordered and unordered |
| Image | full | via URL or upload |
| Link | full | |
| Bold | full | |
| Italic | full | |
| Inline Code | full | |
| Video | limited | via Zhihu video only |

### Constraints

| Field | Limit |
|-------|-------|
| Title | 100 characters |
| Body | 100,000 characters |
| Tags | 5 tags |
| Images | 20 per article |

### Auth Model

OAuth2 flow:

1. redirect to Zhihu login
2. user authorizes
3. receive access token
4. refresh token before expiry

### Rate Limits

- 100 requests per hour
- 10 publish requests per hour

### API Endpoints

- create article: POST /articles
- upload image: POST /images
- get article: GET /articles/{id}

---

## Bilibili

### Content Type

Articles with custom format.

### Supported Elements

| Element | Support | Notes |
|---------|---------|-------|
| Heading | full | |
| Paragraph | full | shorter preferred |
| Code Block | limited | no syntax highlighting |
| Block Quote | full | |
| List | full | |
| Image | full | must upload |
| Link | limited | nofollow |
| Bold | full | |
| Italic | not supported | |
| Inline Code | not supported | |
| Video | full | Bilibili video |

### Constraints

| Field | Limit |
|-------|-------|
| Title | 80 characters |
| Body | 50,000 characters |
| Tags | 10 tags |
| Images | 50 per article |
| Cover | required |

### Auth Model

Cookie-based:

1. user logs in to Bilibili
2. extract cookies
3. use cookies for API calls
4. refresh when expired

### Rate Limits

- 30 requests per hour
- 5 publish requests per hour

### API Endpoints

- create article: POST /x/article/up/create
- upload image: POST /x/article/upcover
- get article: GET /x/article/view

---

## Weibo

### Content Type

Short posts (weibo).

### Supported Elements

| Element | Support | Notes |
|---------|---------|-------|
| Heading | not supported | |
| Paragraph | full | |
| Code Block | not supported | |
| Block Quote | not supported | |
| List | not supported | |
| Image | full | must upload |
| Link | full | auto-shortened |
| Bold | not supported | |
| Italic | not supported | |
| Inline Code | not supported | |
| Video | full | via Weibo video |

### Constraints

| Field | Limit |
|-------|-------|
| Title | not applicable | 
| Body | 2,000 characters |
| Tags | 5 tags |
| Images | 9 per post |

### Auth Model

OAuth2 flow:

1. redirect to Weibo login
2. user authorizes
3. receive access token
4. refresh token before expiry

### Rate Limits

- 200 requests per hour
- 30 publish requests per hour

### API Endpoints

- create post: POST /statuses/share
- upload image: POST /statuses/upload

---

## Content Transformation Rules

### Zhihu

- markdown → HTML
- code blocks → pre/code with class
- images → img tags or upload
- links → a tags

### Bilibili

- markdown → custom block format
- images → must upload, use returned URL
- cover → required, extract or specify
- italic → use bold instead

### Weibo

- markdown → plain text
- images → must upload
- links → auto-shortened
- no formatting support

---

## Validation Matrix

### Per Platform

| Validation | Zhihu | Bilibili | Weibo |
|-----------|-------|----------|-------|
| Title required | yes | yes | no |
| Title max length | 100 | 80 | N/A |
| Body required | yes | yes | yes |
| Body max length | 100K | 50K | 2K |
| Tags max count | 5 | 10 | 5 |
| Cover required | no | yes | no |
| Images max count | 20 | 50 | 9 |

---

## Error Codes Per Platform

### Zhihu

| Code | Meaning | Retryable |
|------|---------|-----------|
| 401 | unauthorized | no |
| 403 | forbidden | no |
| 429 | rate limited | yes |
| 500 | server error | yes |

### Bilibili

| Code | Meaning | Retryable |
|------|---------|-----------|
| -101 | not logged in | no |
| -400 | bad request | no |
| -412 | rate limited | yes |
| 65535 | server error | yes |

### Weibo

| Code | Meaning | Retryable |
|------|---------|-----------|
| 10001 | invalid token | no |
| 10003 | permission denied | no |
| 10024 | rate limited | yes |
| 20000 | server error | yes |

---

## Platform Priority

For MVP:

1. Zhihu (most common for tech content)
2. Bilibili (growing platform)
3. Weibo (optional)

---

## Adding New Platform

To add a new platform:

1. document capabilities in this file
2. implement platform-specific renderer
3. implement platform-specific uploader
4. define validation rules
5. register in platform registry

---

## Non-Goals For MVP

Not included:

- WeChat support
- Juejin support
- Toutiao support
- international platforms
