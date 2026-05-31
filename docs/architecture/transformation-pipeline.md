# Transformation Pipeline

## Goal

Define the content transformation system that converts canonical content into platform-specific payloads.

This document is the source of truth for:

- intermediate representation (IR)
- transformation pipeline
- platform renderers
- extensibility model

---

## Core Principle

Transformation is NOT string conversion.

```
Canonical Markdown
    ↓
Parse to AST
    ↓
Build IR (Intermediate Representation)
    ↓
Platform Renderer
    ↓
Platform Payload
```

This is the real infrastructure.

---

## Why IR Matters

Platforms differ in:

- supported HTML elements
- custom block types
- media embedding
- link handling
- code formatting
- image hosting

Direct string conversion produces:

- inconsistent output
- platform-specific hacks
- unmaintainable code

IR produces:

- clean separation
- testable transforms
- reusable components

---

## Node Types

### Base Interface

```go
type Node interface {
    Type() NodeType
}
```

### Block Nodes

```go
type Heading struct {
    Level int    // 1-6
    Text  string
}

type Paragraph struct {
    Children []InlineNode
}

type CodeBlock struct {
    Language string
    Code     string
}

type BlockQuote struct {
    Children []BlockNode
}

type List struct {
    Ordered  bool
    Items    []ListItem
}

type ListItem struct {
    Children []BlockNode
}

type Image struct {
    URL   string
    Alt   string
    Title string
}

type HorizontalRule struct{}
```

### Inline Nodes

```go
type Text struct {
    Content string
}

type Bold struct {
    Children []InlineNode
}

type Italic struct {
    Children []InlineNode
}

type Code struct {
    Content string
}

type Link struct {
    URL      string
    Text     string
    Title    string
}

type ImageInline struct {
    URL   string
    Alt   string
}
```

---

## Document Model

```go
type Document struct {
    Title    string
    Blocks   []BlockNode
    Metadata map[string]interface{}
}
```

---

## Pipeline Stages

### Stage 1: Parse

```
Markdown string → AST
```

Use goldmark or similar parser.

### Stage 2: Transform

```
AST → Document (IR)
```

Convert parsed AST into our node types.

### Stage 3: Render

```
Document → Platform Payload
```

Each platform has its own renderer.

---

## Parser

```go
type Parser interface {
    Parse(markdown string) (*Document, error)
}
```

Default implementation uses goldmark.

---

## Renderer Interface

```go
type Renderer interface {
    Render(doc *Document) (*PlatformPayload, error)
    SupportedNodes() []NodeType
}
```

---

## Platform Payload

```go
type PlatformPayload struct {
    Title       string
    Body        string      // platform-specific format
    Tags        []string
    Assets      []AssetRef
    Metadata    map[string]interface{}
}
```

---

## Example: Zhihu Renderer

Zhihu supports:

- HTML in article body
- code blocks with syntax highlighting
- images via URL
- custom link formatting

```go
type ZhihuRenderer struct{}

func (r *ZhihuRenderer) Render(doc *Document) (*PlatformPayload, error) {
    var html strings.Builder

    for _, block := range doc.Blocks {
        switch node := block.(type) {
        case *Heading:
            html.WriteString(fmt.Sprintf("<h%d>%s</h%d>", node.Level, node.Text, node.Level))
        case *Paragraph:
            html.WriteString("<p>")
            for _, inline := range node.Children {
                html.WriteString(r.renderInline(inline))
            }
            html.WriteString("</p>")
        case *CodeBlock:
            html.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">%s</code></pre>",
                node.Language, html.EscapeString(node.Code)))
        // ... other cases
        }
    }

    return &PlatformPayload{
        Title: doc.Title,
        Body:  html.String(),
        Tags:  extractTags(doc),
    }, nil
}
```

---

## Example: Bilibili Renderer

Bilibili has:

- custom article format
- shorter paragraphs preferred
- topic tags system
- cover image required

```go
type BilibiliRenderer struct{}

func (r *BilibiliRenderer) Render(doc *Document) (*PlatformPayload, error) {
    // Bilibili uses different format
    var content []BilibiliBlock

    for _, block := range doc.Blocks {
        switch node := block.(type) {
        case *Heading:
            content = append(content, BilibiliBlock{
                Type: "heading",
                Text: node.Text,
                Level: node.Level,
            })
        case *Paragraph:
            content = append(content, BilibiliBlock{
                Type: "text",
                Text: r.renderPlainText(node),
            })
        // ... other cases
        }
    }

    return &PlatformPayload{
        Title:    doc.Title,
        Body:     r.serializeBlocks(content),
        Tags:     extractTags(doc),
        Metadata: map[string]interface{}{"cover": extractCover(doc)},
    }, nil
}
```

---

## Asset Handling

During transformation:

```
Image nodes → Asset references
```

```go
type AssetRef struct {
    ID       string
    URL      string
    Type     AssetType
    Original string  // original markdown reference
}
```

Renderers collect asset references.

Asset pipeline handles upload separately.

---

## NodeVisitor Pattern

For extensibility:

```go
type NodeVisitor interface {
    VisitHeading(*Heading) error
    VisitParagraph(*Paragraph) error
    VisitCodeBlock(*CodeBlock) error
    VisitBlockQuote(*BlockQuote) error
    VisitList(*List) error
    VisitImage(*Image) error
    // ... other nodes
}
```

Renderers implement NodeVisitor.

---

## Testing

Each renderer is independently testable:

```go
func TestZhihuRenderer_Heading(t *testing.T) {
    doc := &Document{
        Blocks: []BlockNode{
            &Heading{Level: 1, Text: "Title"},
        },
    }

    renderer := &ZhihuRenderer{}
    payload, err := renderer.Render(doc)

    assert.NoError(t, err)
    assert.Contains(t, payload.Body, "<h1>Title</h1>")
}
```

---

## Adding New Platform

1. implement Renderer interface
2. define node handling for each supported node
3. register renderer in registry

No changes to parser or IR.

---

## Non-Goals For MVP

Not included:

- AI-powered transformation
- custom block types per platform
- rich media embedding
- interactive content
- real-time preview
