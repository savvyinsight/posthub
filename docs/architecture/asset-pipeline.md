# Asset Pipeline

## Goal

Define the asset handling system for content publishing.

This document is the source of truth for:

- asset types
- asset storage
- asset upload flow
- asset reference rewriting

---

## Why Assets Matter

Real publishing platforms require:

- images
- cover images
- videos
- attachments

Publishing without assets produces incomplete content.

---

## Asset Types

```go
type AssetType string

const (
    AssetTypeImage    AssetType = "image"
    AssetTypeVideo    AssetType = "video"
    AssetTypeCover    AssetType = "cover"
    AssetTypeDocument AssetType = "document"
)
```

---

## Asset Model

```go
type Asset struct {
    ID           string
    Type         AssetType
    OriginalURL  string
    LocalPath    string
    ContentType  string
    Size         int64
    Checksum     string
    CreatedAt    time.Time
}
```

---

## Platform Asset Models

Each platform handles assets differently.

| Platform | Image Upload | Cover | Video |
|----------|-------------|-------|-------|
| Zhihu | URL or upload | optional | limited |
| Bilibili | upload only | required | supported |
| Weibo | upload only | optional | supported |

---

## Asset Flow

### During Content Creation

```
User writes markdown with image references
    ↓
Images stored as URLs in canonical content
    ↓
No local storage yet
```

### During Publishing

Important: asset upload must happen BEFORE rendering.

Why:

- uploaded image dimensions may affect layout
- platform media IDs must be embedded in content
- transformed URLs must be available for renderer

Correct flow:

```
Parse content for asset references
    ↓
Collect all asset references
    ↓
Deduplicate assets (by URL)
    ↓
Download assets to local storage
    ↓
Upload assets to target platform
    ↓
Get platform asset IDs
    ↓
Inject platform asset refs into document
    ↓
Render document (with platform asset refs)
    ↓
Publish rendered payload
```

---

## Asset Reference in Content

Markdown references:

```markdown
![Alt text](https://example.com/image.jpg)
```

In IR:

```go
type Image struct {
    URL        string
    Alt        string
    AssetRef   *AssetReference
}

type AssetReference struct {
    OriginalURL string
    AssetID     string        // set after upload
    PlatformID  string        // set after platform upload
}
```

---

## Asset Storage

### Local Storage

```
/tmp/posthub/assets/{asset_id}
```

Temporary storage during publish.

Cleaned up after publish completes.

### No Permanent Storage in MVP

MVP does not store assets permanently.

Assets are:

1. downloaded from source URL
2. uploaded to platform
3. deleted locally

---

## Asset Upload Interface

```go
type AssetUploader interface {
    Upload(ctx context.Context, asset *Asset) (*PlatformAssetID, error)
    SupportsType(assetType AssetType) bool
}
```

---

## Platform Asset Handling

### Zhihu

- images: upload via API, get image ID
- cover: extract from first image or specify
- body: HTML with image IDs

### Bilibili

- images: upload via API, get image URL
- cover: required, must upload
- body: custom format with image references

---

## Asset Collection

During transformation:

```go
type AssetCollector struct {
    assets []AssetReference
}

func (c *AssetCollector) VisitImage(img *Image) error {
    c.assets = append(c.assets, AssetReference{
        OriginalURL: img.URL,
    })
    return nil
}
```

Returns list of assets needed for publish.

---

## Asset Deduplication

Important: same image may appear multiple times in content.

Example:

```markdown
![Logo](https://example.com/logo.png)
![Logo](https://example.com/logo.png)
```

Without deduplication: upload twice, waste bandwidth.

### Deduplication Strategy

```go
func deduplicateAssets(assets []AssetReference) []AssetReference {
    seen := make(map[string]bool)
    result := []AssetReference{}

    for _, asset := range assets {
        if !seen[asset.OriginalURL] {
            seen[asset.OriginalURL] = true
            result = append(result, asset)
        }
    }

    return result
}
```

### Mapping After Upload

After upload, map original URLs to platform IDs:

```go
type AssetMapping struct {
    OriginalURL string
    PlatformID  string
    LocalPath   string
}
```

Document nodes reference mapping by original URL.

---

## Asset Processing Pipeline

### Correct Order

```
Parse document
    ↓
Collect asset references from document
    ↓
Deduplicate assets by URL
    ↓
For each unique asset:
    Download from source URL
    Validate file type and size
    Store locally
    ↓
For each platform:
    Upload assets via platform uploader
    Get platform asset IDs
    ↓
Inject platform asset refs into document
    ↓
Pass document to renderer
    ↓
Renderer uses platform asset refs
    ↓
Cleanup temporary files
```

### Why This Order

1. Collect before download: avoid unnecessary downloads if no assets
2. Deduplicate before download: avoid downloading same asset twice
3. Upload before render: renderer needs platform asset IDs
4. Inject before render: document must contain platform refs
5. Cleanup after render: files only needed during publish

---

## Error Handling

### Download Failure

- retry download (max 3)
- if still fails: fail publish task
- error: "failed to download asset"

### Upload Failure

- retry upload (max 3)
- if still fails: fail publish task
- error: "failed to upload asset to platform"

### Validation Failure

- invalid file type: fail immediately
- file too large: fail immediately
- error: "asset validation failed"

---

## Concurrency

Assets are uploaded concurrently:

```go
func uploadAssets(ctx context.Context, assets []Asset, uploader AssetUploader) error {
    g, ctx := errgroup.WithContext(ctx)
    for i := range assets {
        asset := &assets[i]
        g.Go(func() error {
            id, err := uploader.Upload(ctx, asset)
            if err != nil {
                return err
            }
            asset.PlatformID = id
            return nil
        })
    }
    return g.Wait()
}
```

---

## Security

### URL Validation

- only allow http/https URLs
- validate URL format
- block internal IPs (SSRF prevention)

### File Validation

- check content type matches extension
- validate file headers
- limit file size (10MB default)

---

## Configuration

```yaml
assets:
  temp_dir: /tmp/posthub/assets
  max_size: 10485760  # 10MB
  allowed_types:
    - image/jpeg
    - image/png
    - image/gif
    - image/webp
  download_timeout: 30s
  upload_timeout: 60s
```

---

## Non-Goals For MVP

Not included:

- permanent asset storage
- CDN integration
- asset transcoding
- video processing
- asset deduplication
- asset versioning

---

## Future Considerations

Post-MVP:

- permanent asset storage (S3/MinIO)
- asset CDN
- asset deduplication by checksum
- video transcoding
- asset metadata extraction
