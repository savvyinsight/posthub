# Platform Abstraction

## Goal

Define the platform interface contract.

This document is the source of truth for:

- platform interface
- adapter responsibilities
- extensibility model
- testing strategy

---

## Core Principle

Every platform implements the same interface.

Adding a new platform means:

1. implement interface
2. register adapter
3. configure credentials

No changes to core system.

---

## Platform Interface

```go
type Platform interface {
    // Name returns the platform identifier
    Name() string

    // Validate checks if content meets platform requirements
    Validate(doc *transform.Document) error

    // UploadAssets uploads assets and returns platform-specific references
    UploadAssets(ctx context.Context, assets []transform.AssetRef) ([]transform.AssetRef, error)

    // Publish publishes content to the platform
    Publish(ctx context.Context, doc *transform.Document, creds *auth.Credentials) (*PublishResult, error)

    // Delete removes published content from the platform
    Delete(ctx context.Context, postID string, creds *auth.Credentials) error

    // Capabilities returns platform capabilities
    Capabilities() Capabilities
}
```

Note: RefreshAuth is NOT in Platform interface.

Auth lifecycle belongs to AuthProvider, not Platform.

Platform receives credentials, does not own them.

---

## Capabilities Model

```go
type Capabilities struct {
    SupportedNodes []transform.NodeType
    MaxTitleLength int
    MaxBodyLength  int
    MaxTags        int
    MaxImages      int
    RequiresCover  bool
    SupportsVideo  bool
    AuthType       AuthType
}
```

---

## Publish Result

```go
type PublishResult struct {
    PlatformPostID string
    PlatformURL    string
    PublishedAt    time.Time
    Response       json.RawMessage
}
```

---

## Adapter Structure

Each platform adapter lives in its own package:

```
internal/platforms/
    zhihu/
        adapter.go
        renderer.go
        uploader.go
        validator.go
    bilibili/
        adapter.go
        renderer.go
        uploader.go
        validator.go
    weibo/
        adapter.go
        renderer.go
        uploader.go
        validator.go
```

---

## Adapter Implementation Example

```go
package zhihu

type Adapter struct {
    auth      auth.AuthProvider
    client    *http.Client
    renderer  *Renderer
    validator *Validator
    uploader  *Uploader
    caps      platform.Capabilities
}

func NewAdapter(authProvider auth.AuthProvider) *Adapter {
    return &Adapter{
        auth:      authProvider,
        client:    &http.Client{Timeout: 30 * time.Second},
        renderer:  NewRenderer(),
        validator: NewValidator(),
        uploader:  NewUploader(),
        caps: platform.Capabilities{
            SupportedNodes: []transform.NodeType{
                transform.NodeHeading,
                transform.NodeParagraph,
                transform.NodeCodeBlock,
                transform.NodeBlockQuote,
                transform.NodeList,
                transform.NodeImage,
                transform.NodeLink,
                transform.NodeBold,
                transform.NodeItalic,
                transform.NodeInlineCode,
            },
            MaxTitleLength: 100,
            MaxBodyLength:  100000,
            MaxTags:        5,
            MaxImages:      20,
            RequiresCover:  false,
            SupportsVideo:  false,
            AuthType:       platform.AuthTypeOAuth2,
        },
    }
}

func (a *Adapter) Name() string {
    return "zhihu"
}

func (a *Adapter) Validate(doc *transform.Document) error {
    return a.validator.Validate(doc, a.caps)
}

func (a *Adapter) UploadAssets(ctx context.Context, assets []transform.AssetRef) ([]transform.AssetRef, error) {
    creds, err := a.auth.Authenticate(ctx)
    if err != nil {
        return nil, err
    }
    return a.uploader.Upload(ctx, assets, creds)
}

func (a *Adapter) Publish(ctx context.Context, doc *transform.Document, creds *auth.Credentials) (*platform.PublishResult, error) {
    // Render document to platform payload
    payload, err := a.renderer.Render(doc)
    if err != nil {
        return nil, fmt.Errorf("render: %w", err)
    }

    // Call platform API
    resp, err := a.callAPI(ctx, payload, creds)
    if err != nil {
        return nil, fmt.Errorf("api call: %w", err)
    }

    return &platform.PublishResult{
        PlatformPostID: resp.ID,
        PlatformURL:    resp.URL,
        PublishedAt:    time.Now(),
        Response:       resp.Raw,
    }, nil
}

func (a *Adapter) Delete(ctx context.Context, postID string, creds *auth.Credentials) error {
    return a.deletePost(ctx, postID, creds)
}

func (a *Adapter) Capabilities() platform.Capabilities {
    return a.caps
}
```

---

## Platform Registry

```go
type Registry struct {
    platforms map[string]Platform
}

func NewRegistry() *Registry {
    return &Registry{
        platforms: make(map[string]Platform),
    }
}

func (r *Registry) Register(p Platform) {
    r.platforms[p.Name()] = p
}

func (r *Registry) Get(name string) (Platform, error) {
    p, ok := r.platforms[name]
    if !ok {
        return nil, fmt.Errorf("platform not found: %s", name)
    }
    return p, nil
}

func (r *Registry) List() []string {
    names := make([]string, 0, len(r.platforms))
    for name := range r.platforms {
        names = append(names, name)
    }
    return names
}
```

---

## Validation Separation

Validation is separate from adapter:

```go
type Validator struct{}

func (v *Validator) Validate(doc *transform.Document, caps platform.Capabilities) error {
    // Check title length
    if len(doc.Title) > caps.MaxTitleLength {
        return fmt.Errorf("title exceeds max length %d", caps.MaxTitleLength)
    }

    // Check body length
    bodyLength := v.calculateBodyLength(doc)
    if bodyLength > caps.MaxBodyLength {
        return fmt.Errorf("body exceeds max length %d", caps.MaxBodyLength)
    }

    // Check supported nodes
    for _, node := range doc.Blocks {
        if !v.isNodeSupported(node.Type(), caps.SupportedNodes) {
            return fmt.Errorf("unsupported node type: %s", node.Type())
        }
    }

    // Check tags
    if len(doc.Tags) > caps.MaxTags {
        return fmt.Errorf("tags exceed max count %d", caps.MaxTags)
    }

    return nil
}
```

---

## Renderer Separation

Renderer only handles transformation:

```go
type Renderer struct{}

func (r *Renderer) Render(doc *transform.Document) (*platform.PlatformPayload, error) {
    var html strings.Builder

    for _, block := range doc.Blocks {
        switch node := block.(type) {
        case *transform.Heading:
            html.WriteString(r.renderHeading(node))
        case *transform.Paragraph:
            html.WriteString(r.renderParagraph(node))
        case *transform.CodeBlock:
            html.WriteString(r.renderCodeBlock(node))
        // ... other cases
        }
    }

    return &platform.PlatformPayload{
        Title: doc.Title,
        Body:  html.String(),
        Tags:  doc.Tags,
    }, nil
}
```

---

## IR Evolution Path

Important: current IR is syntax-oriented MVP IR.

Current IR models:

- headings
- paragraphs
- code blocks
- bold/italic
- links
- images

This is presentation-oriented.

### Future: Semantic IR

Future evolution:

```
Markdown
    ↓
Syntax AST
    ↓
Semantic IR
    ↓
Platform Adaptation
    ↓
Render IR
    ↓
Payload
```

Semantic nodes:

```go
type Section struct {
    Title    string
    Summary  string
    Children []Node
}

type Callout struct {
    Type     string  // "warning", "info", "tip"
    Content  string
}

type CodeExample struct {
    Language  string
    Code      string
    Caption   string
}

type TutorialStep struct {
    Number    int
    Title     string
    Content   string
}

type ReferenceLink struct {
    URL       string
    Title     string
    Context   string
}

type Summary struct {
    Content   string
}
```

Why semantic IR matters:

- platform adaptation becomes semantic
- AI transformation becomes possible
- content reuse across platforms
- better validation

### Current vs Future

| Aspect | Current (MVP) | Future |
|--------|---------------|--------|
| IR type | Syntax nodes | Semantic nodes |
| Rendering | Format conversion | Content adaptation |
| AI integration | Difficult | Natural |
| Platform adaptation | Visual only | Semantic + visual |

Document this evolution explicitly.

Do NOT overbuild now.

But design for this path.

---

## Uploader Separation

Uploader handles asset uploads:

```go
type Uploader struct {
    client *http.Client
}

func (u *Uploader) Upload(ctx context.Context, assets []transform.AssetRef, creds *auth.Credentials) ([]transform.AssetRef, error) {
    results := make([]transform.AssetRef, len(assets))

    for i, asset := range assets {
        platformID, err := u.uploadSingle(ctx, asset, creds)
        if err != nil {
            return nil, fmt.Errorf("upload asset %s: %w", asset.OriginalURL, err)
        }

        results[i] = transform.AssetRef{
            OriginalURL: asset.OriginalURL,
            PlatformID:  platformID,
            Type:        asset.Type,
        }
    }

    return results, nil
}
```

---

## Testing Strategy

### Unit Tests

Each component tested independently:

```go
func TestValidator_MaxTitleLength(t *testing.T) {
    v := NewValidator()
    caps := platform.Capabilities{MaxTitleLength: 100}

    doc := &transform.Document{
        Title: strings.Repeat("a", 101),
    }

    err := v.Validate(doc, caps)
    assert.Error(t, err)
}
```

### Integration Tests

Test adapter with mock API:

```go
func TestAdapter_Publish(t *testing.T) {
    mockAPI := NewMockAPI()
    adapter := NewAdapter(mockAPI)

    doc := &transform.Document{
        Title: "Test",
        Body:  "Content",
    }

    result, err := adapter.Publish(context.Background(), doc, testCreds)
    assert.NoError(t, err)
    assert.NotEmpty(t, result.PlatformPostID)
}
```

---

## Adding New Platform

### Steps

1. create package: `internal/platforms/newplatform/`
2. implement `Platform` interface
3. create renderer for platform format
4. create validator for platform constraints
5. create uploader for platform assets
6. register in main:

```go
registry.Register(newplatform.NewAdapter(authProvider))
```

### No Core Changes Required

The core system:

- does not know about specific platforms
- uses only the Platform interface
- routes tasks by platform name

---

## Non-Goals For MVP

Not included:

- plugin system
- dynamic platform loading
- platform SDK
- platform testing sandbox
