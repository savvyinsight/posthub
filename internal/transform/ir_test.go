package transform

import "testing"

func TestNodeTypes(t *testing.T) {
	// Verify block nodes implement BlockNode
	var _ BlockNode = &Heading{}
	var _ BlockNode = &Paragraph{}
	var _ BlockNode = &CodeBlock{}
	var _ BlockNode = &BlockQuote{}
	var _ BlockNode = &List{}
	var _ BlockNode = &Image{}
	var _ BlockNode = &HorizontalRule{}

	// Verify inline nodes implement InlineNode
	var _ InlineNode = &Text{}
	var _ InlineNode = &Bold{}
	var _ InlineNode = &Italic{}
	var _ InlineNode = &InlineCode{}
	var _ InlineNode = &Link{}
	var _ InlineNode = &ImageInline{}
}

func TestHeading_Type(t *testing.T) {
	h := &Heading{Level: 1, Text: "Title"}
	if h.Type() != NodeHeading {
		t.Errorf("Heading.Type() = %s, want %s", h.Type(), NodeHeading)
	}
}

func TestDocument_Construction(t *testing.T) {
	doc := &Document{
		Title: "Test Document",
		Blocks: []BlockNode{
			&Heading{Level: 1, Text: "Hello"},
			&Paragraph{
				Children: []InlineNode{
					&Text{Content: "World"},
				},
			},
			&CodeBlock{Language: "go", Code: "fmt.Println()"},
		},
		Tags: []string{"go", "test"},
	}

	if doc.Title != "Test Document" {
		t.Errorf("Title = %s, want 'Test Document'", doc.Title)
	}
	if len(doc.Blocks) != 3 {
		t.Errorf("len(Blocks) = %d, want 3", len(doc.Blocks))
	}
	if doc.Blocks[0].Type() != NodeHeading {
		t.Errorf("first block type = %s, want %s", doc.Blocks[0].Type(), NodeHeading)
	}
}
