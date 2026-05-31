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

### Implementation

```go
type CookieProvider struct {
    name     string
    config   CookieConfig
    cookies  []*http.Cookie
    mu       sync.RWMutex
    store    CookieStore
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

func (p *CookieProvider) IsExpired() bool {
    p.mu.RLock()
    defer p.mu.RUnlock()

    // Check if any cookie is expired
    for _, c := range p.cookies {
        if c.Expires.Before(time.Now()) {
            return true
        }
    }
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
    platform VARCHAR(50) NOT NULL UNIQUE,
    credential_type VARCHAR(20) NOT NULL,
    encrypted_data BYTEA NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_credentials_platform ON platform_credentials(platform);
```

### Encryption

Credentials encrypted at rest using AES-256.

Encryption key from environment:

```env
CREDENTIAL_ENCRYPTION_KEY=base64-encoded-key
```

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
