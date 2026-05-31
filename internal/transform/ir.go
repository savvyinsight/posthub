// Package transform defines the Intermediate Representation (IR) for content
// and the transformation pipeline that converts markdown into platform-specific payloads.
//
// The IR is the bridge between canonical markdown content and platform renderers.
// Each platform implements a Renderer that consumes IR nodes and produces
// platform-specific output.
//
// Current IR is syntax-oriented (MVP). Future evolution will introduce
// semantic IR nodes (Section, Callout, CodeExample, etc.) for richer
// platform adaptation and AI-powered transformation.
package transform

import "time"

// NodeType identifies the kind of IR node.
type NodeType string

const (
	// Block nodes
	NodeHeading        NodeType = "heading"
	NodeParagraph      NodeType = "paragraph"
	NodeCodeBlock      NodeType = "code_block"
	NodeBlockQuote     NodeType = "blockquote"
	NodeList           NodeType = "list"
	NodeImage          NodeType = "image"
	NodeHorizontalRule NodeType = "horizontal_rule"

	// Inline nodes
	NodeText        NodeType = "text"
	NodeBold        NodeType = "bold"
	NodeItalic      NodeType = "italic"
	NodeInlineCode  NodeType = "inline_code"
	NodeLink        NodeType = "link"
	NodeImageInline NodeType = "image_inline"
)

// BlockNode is a block-level IR node (heading, paragraph, code block, etc.).
type BlockNode interface {
	Type() NodeType
}

// InlineNode is an inline-level IR node (text, bold, italic, etc.).
type InlineNode interface {
	Type() NodeType
}

// --- Block Nodes ---

// Heading represents a markdown heading (h1-h6).
type Heading struct {
	Level int
	Text  string
}

func (h *Heading) Type() NodeType { return NodeHeading }

// Paragraph represents a paragraph containing inline nodes.
type Paragraph struct {
	Children []InlineNode
}

func (p *Paragraph) Type() NodeType { return NodeParagraph }

// CodeBlock represents a fenced code block with optional language.
type CodeBlock struct {
	Language string
	Code     string
}

func (c *CodeBlock) Type() NodeType { return NodeCodeBlock }

// BlockQuote represents a blockquote containing block nodes.
type BlockQuote struct {
	Children []BlockNode
}

func (b *BlockQuote) Type() NodeType { return NodeBlockQuote }

// List represents an ordered or unordered list.
type List struct {
	Ordered bool
	Items   []ListItem
}

func (l *List) Type() NodeType { return NodeList }

// ListItem represents a single item in a list.
type ListItem struct {
	Children []BlockNode
}

// Image represents a block-level image.
type Image struct {
	URL   string
	Alt   string
	Title string
}

func (i *Image) Type() NodeType { return NodeImage }

// HorizontalRule represents a thematic break (---).
type HorizontalRule struct{}

func (h *HorizontalRule) Type() NodeType { return NodeHorizontalRule }

// --- Inline Nodes ---

// Text represents plain text content.
type Text struct {
	Content string
}

func (t *Text) Type() NodeType { return NodeText }

// Bold represents bold text (e.g., **text**).
type Bold struct {
	Children []InlineNode
}

func (b *Bold) Type() NodeType { return NodeBold }

// Italic represents italic text (e.g., *text*).
type Italic struct {
	Children []InlineNode
}

func (i *Italic) Type() NodeType { return NodeItalic }

// InlineCode represents inline code (e.g., `code`).
type InlineCode struct {
	Content string
}

func (c *InlineCode) Type() NodeType { return NodeInlineCode }

// Link represents a hyperlink.
type Link struct {
	URL   string
	Text  string
	Title string
}

func (l *Link) Type() NodeType { return NodeLink }

// ImageInline represents an inline image (e.g., ![alt](url)).
type ImageInline struct {
	URL string
	Alt string
}

func (i *ImageInline) Type() NodeType { return NodeImageInline }

// --- Document Model ---

// Document represents a parsed markdown document as IR.
type Document struct {
	Title    string
	Blocks   []BlockNode
	Tags     []string
	Metadata map[string]any
}

// --- Asset References ---

// AssetType classifies an asset.
type AssetType string

const (
	AssetTypeImage AssetType = "image"
	AssetTypeVideo AssetType = "video"
	AssetTypeFile  AssetType = "file"
)

// AssetRef represents a reference to an asset that may need uploading.
type AssetRef struct {
	ID          string    `json:"id,omitempty"`
	URL         string    `json:"url"`
	OriginalURL string    `json:"original_url,omitempty"`
	PlatformID  string    `json:"platform_id,omitempty"`
	Type        AssetType `json:"type"`
}

// --- Platform Payload ---

// PlatformPayload is the rendered output for a specific platform.
type PlatformPayload struct {
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Tags     []string       `json:"tags,omitempty"`
	Assets   []AssetRef     `json:"assets,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// --- Interfaces ---

// Parser converts markdown into an IR Document.
type Parser interface {
	Parse(markdown string) (*Document, error)
}

// Renderer converts an IR Document into a platform-specific payload.
type Renderer interface {
	Render(doc *Document) (*PlatformPayload, error)
	SupportedNodes() []NodeType
}

// --- Content Model (for transformation context) ---

// Content represents content being transformed.
type Content struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
