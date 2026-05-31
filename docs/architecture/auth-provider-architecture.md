# Auth Provider Architecture

## Goal

Define how the system manages platform authentication.

This document is the source of truth for:

- credential storage
- token management
- auth provider abstraction
- multi-account support (future)

---

## Core Problem

Each platform has different auth:

- Zhihu: OAuth2
- Bilibili: Cookie-based
- Weibo: OAuth2

System must:

- store credentials securely
- refresh tokens automatically
- isolate credentials per platform
- support multiple accounts (future)

---

## Architecture

```
┌─────────────────┐
│  Auth Registry  │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────┐
│ OAuth2 │ │ Cookie │
│Provider│ │Provider│
└────────┘ └────────┘
```

---

## Auth Provider Interface

```go
type AuthProvider interface {
    // Name returns the provider identifier
    Name() string

    // Authenticate returns valid credentials for API calls
    Authenticate(ctx context.Context) (*Credentials, error)

    // Refresh refreshes expired credentials
    Refresh(ctx context.Context) error

    // IsExpired checks if credentials are expired
    IsExpired() bool

    // Validate checks if credentials are valid
    Validate(ctx context.Context) error
}
```

---

## Credentials Model

```go
type Credentials struct {
    AccessToken  string
    RefreshToken string
    TokenType    string
    ExpiresAt    time.Time
    Cookies      []*http.Cookie
    Metadata     map[string]string
}
```

---

## OAuth2 Provider

For platforms using OAuth2 (Zhihu, Weibo).

### Configuration

```go
type OAuth2Config struct {
    ClientID     string
    ClientSecret string
    AuthURL      string
    TokenURL     string
    RedirectURL  string
    Scopes       []string
}
```

### Token Storage

Tokens stored in:

- database (encrypted)
- or config file (development)

### Token Refresh Flow

```
Check if token expired
    ↓
If expired: call refresh endpoint
    ↓
Update stored token
    ↓
Return new credentials
```

### Implementation

```go
type OAuth2Provider struct {
    name      string
    config    OAuth2Config
    token     *oauth2.Token
    tokenMu   sync.RWMutex
    store     TokenStore
}

func (p *OAuth2Provider) Authenticate(ctx context.Context) (*Credentials, error) {
    p.tokenMu.RLock()
    if p.token != nil && p.token.Valid() {
        creds := p.tokenToCredentials(p.token)
        p.tokenMu.RUnlock()
        return creds, nil
    }
    p.tokenMu.RUnlock()

    // Need to refresh
    if err := p.Refresh(ctx); err != nil {
        return nil, err
    }

    p.tokenMu.RLock()
    defer p.tokenMu.RUnlock()
    return p.tokenToCredentials(p.token), nil
}

func (p *OAuth2Provider) Refresh(ctx context.Context) error {
    p.tokenMu.Lock()
    defer p.tokenMu.Unlock()

    newToken, err := p.config.TokenSource(ctx, p.token).Token()
    if err != nil {
        return fmt.Errorf("refresh token: %w", err)
    }

    p.token = newToken
    return p.store.Save(ctx, p.name, newToken)
}
```

---

## Cookie Provider

For platforms using cookies (Bilibili).

### Configuration

```go
type CookieConfig struct {
    Cookies []*http.Cookie
    Domain  string
}
```

### Cookie Storage

Cookies stored in:

- database (encrypted)
- or config file (development)

### Cookie Refresh

Cookie refresh requires:

- user re-login
- or automated browser session (future)

MVP: manual refresh.

### Cookie Validity

Important: cookie expiration timestamps are unreliable.

Many real cookies:

- have zero expiration
- are session cookies
- rely on server invalidation

Correct model:

```
cookie validity = determined by API response
```

Not local expiration timestamps.

Validation approach:

1. use cookies for API call
2. if API returns auth error → cookies invalid
3. if API succeeds → cookies valid
4. cache validity result with short TTL

### Implementation

```go
type CookieProvider struct {
    name        string
    config      CookieConfig
    cookies     []*http.Cookie
    mu          sync.RWMutex
    store       CookieStore
    lastCheck   time.Time
    lastValid   bool
}

func (p *CookieProvider) Authenticate(ctx context.Context) (*Credentials, error) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if p.cookies == nil {
        return nil, ErrNotAuthenticated
    }

    return &Credentials{
        Cookies:  p.cookies,
        Metadata: map[string]string{"domain": p.config.Domain},
    }, nil
}

func (p *CookieProvider) Validate(ctx context.Context) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    // Cache validity for 5 minutes
    if time.Since(p.lastCheck) < 5*time.Minute {
        if p.lastValid {
            return nil
        }
        return ErrInvalidCredentials
    }

    // Test cookies by making lightweight API call
    err := p.testCookies(ctx)
    p.lastCheck = time.Now()
    p.lastValid = err == nil

    return err
}

func (p *CookieProvider) IsExpired() bool {
    // Do NOT rely on cookie expiration timestamps
    // Use Validate() instead
    return false
}
```

---

## Token Store Interface

```go
type TokenStore interface {
    Save(ctx context.Context, provider string, token *oauth2.Token) error
    Load(ctx context.Context, provider string) (*oauth2.Token, error)
    Delete(ctx context.Context, provider string) error
}
```

---

## Cookie Store Interface

```go
type CookieStore interface {
    Save(ctx context.Context, provider string, cookies []*http.Cookie) error
    Load(ctx context.Context, provider string) ([]*http.Cookie, error)
    Delete(ctx context.Context, provider string) error
}
```

---

## Database Storage

### platform_credentials table

```sql
CREATE TABLE platform_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    platform VARCHAR(50) NOT NULL,
    credential_type VARCHAR(20) NOT NULL,
    encrypted_data BYTEA NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_platform_credentials_account_platform UNIQUE (account_id, platform)
);

CREATE INDEX idx_platform_credentials_account ON platform_credentials(account_id);
CREATE INDEX idx_platform_credentials_platform ON platform_credentials(platform);
```

Why account_id:

- supports multiple accounts per platform (future)
- supports team accounts (future)
- supports organization accounts (future)
- even if MVP only has one account per user

Do NOT bake incorrect cardinality into schema.

### Encryption

Credentials encrypted at rest using AES-256-GCM.

Encryption details:

- algorithm: AES-256-GCM
- nonce: random 12 bytes per encryption operation
- key: 256-bit key from environment
- authentication: GCM provides authenticated encryption

Encryption key from environment:

```env
CREDENTIAL_ENCRYPTION_KEY=base64-encoded-32-bytes
```

Key rotation strategy:

- store key version with encrypted data
- support multiple active keys
- re-encrypt on rotation

---

## Auth Registry

```go
type AuthRegistry struct {
    providers map[string]AuthProvider
}

func (r *AuthRegistry) Register(provider AuthProvider) {
    r.providers[provider.Name()] = provider
}

func (r *AuthRegistry) Get(platform string) (AuthProvider, error) {
    p, ok := r.providers[platform]
    if !ok {
        return nil, fmt.Errorf("no auth provider for platform: %s", platform)
    }
    return p, nil
}
```

---

## Credential Ownership Boundary

Important: who owns credentials?

Workers should NEVER directly access credential storage.

Correct ownership chain:

```
workflow
    ↓
platform adapter
    ↓
auth provider
    ↓
credential store
```

Flow:

1. workflow triggers publish
2. platform adapter requests credentials
3. auth provider retrieves from store
4. auth provider returns to adapter
5. adapter uses credentials for API call

Workers do NOT:

- know about credential storage
- access credential store directly
- manage token lifecycle

---

## Integration with Publishers

Publishers use auth providers:

```go
type Publisher struct {
    auth     AuthProvider
    renderer Renderer
}

func (p *Publisher) Publish(ctx context.Context, doc *Document) (*Result, error) {
    creds, err := p.auth.Authenticate(ctx)
    if err != nil {
        return nil, fmt.Errorf("authenticate: %w", err)
    }

    // Use creds for API call
    return p.doPublish(ctx, doc, creds)
}
```

---

## Error Handling

### Auth Errors

```go
var (
    ErrNotAuthenticated  = errors.New("not authenticated")
    ErrTokenExpired      = errors.New("token expired")
    ErrRefreshFailed     = errors.New("token refresh failed")
    ErrInvalidCredentials = errors.New("invalid credentials")
)
```

### Error Classification

- ErrNotAuthenticated: permanent, user action required
- ErrTokenExpired: retryable after refresh
- ErrRefreshFailed: permanent, user action required
- ErrInvalidCredentials: permanent, user action required

---

## Multi-Account Support (Future)

Future architecture:

```go
type AccountAuth struct {
    AccountID string
    Provider  AuthProvider
}
```

Each publish task references specific account.

---

## Configuration

```yaml
auth:
  encryption_key: ${CREDENTIAL_ENCRYPTION_KEY}
  providers:
    zhihu:
      type: oauth2
      client_id: ${ZHIHU_CLIENT_ID}
      client_secret: ${ZHIHU_CLIENT_SECRET}
      token_url: https://www.zhihu.com/oauth2/token
    bilibili:
      type: cookie
      cookies: ${BILIBILI_COOKIES}
    weibo:
      type: oauth2
      client_id: ${WEIBO_CLIENT_ID}
      client_secret: ${WEIBO_CLIENT_SECRET}
      token_url: https://api.weibo.com/oauth2/access_token
```

---

## Security Considerations

### Credential Isolation

- each platform has separate credentials
- credentials never cross platform boundaries
- credentials encrypted at rest

### Access Control

- credentials only accessible by auth provider
- publishers receive temporary credentials
- credentials not logged

### Rotation

- tokens refreshed automatically
- cookies require manual refresh (MVP)
- encryption key rotatable

---

## Non-Goals For MVP

Not included:

- automated browser login
- multi-account per platform
- credential sharing
- SSO integration
- API key authentication
