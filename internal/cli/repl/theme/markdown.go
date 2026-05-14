package theme

import (
	"strings"

	"github.com/charmbracelet/glamour/ansi"
)

const markdownMargin = 2

// MarkdownStyleConfig keeps assistant markdown on the terminal's default
// foreground color while preserving markdown structure.
func MarkdownStyleConfig(wordWrap int) ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
			},
			Margin: uintPtr(markdownMargin),
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      uintPtr(1),
			IndentToken: stringPtr("| "),
		},
		Paragraph: ansi.StyleBlock{},
		List: ansi.StyleList{
			LevelIndent: 4,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Format: "\n" + strings.Repeat("─", markdownContentWidth(wordWrap)) + "\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
		},
		Task: ansi.StyleTask{
			Ticked:   "[x] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Format: "Image: {{.text}} ->",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "`",
				Suffix: "`",
				Color:  stringPtr("#7986CB"),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				Margin: uintPtr(markdownMargin),
			},
			Chroma: markdownChromaStyle(),
		},
		Table: ansi.StyleTable{
			CenterSeparator: stringPtr("┼"),
			ColumnSeparator: stringPtr("│"),
			RowSeparator:    stringPtr("─"),
		},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n* ",
		},
	}
}

func markdownContentWidth(wordWrap int) int {
	width := wordWrap - markdownMargin*2
	if width < 1 {
		return 1
	}
	return width
}

func markdownChromaStyle() *ansi.Chroma {
	return &ansi.Chroma{
		Comment: ansi.StylePrimitive{
			Color:  stringPtr("#78909C"),
			Italic: boolPtr(true),
		},
		CommentPreproc: ansi.StylePrimitive{
			Color: stringPtr("#FFB74D"),
		},
		Keyword: ansi.StylePrimitive{
			Color: stringPtr("#BA68C8"),
			Bold:  boolPtr(true),
		},
		KeywordReserved: ansi.StylePrimitive{
			Color: stringPtr("#BA68C8"),
			Bold:  boolPtr(true),
		},
		KeywordNamespace: ansi.StylePrimitive{
			Color: stringPtr("#BA68C8"),
		},
		KeywordType: ansi.StylePrimitive{
			Color: stringPtr("#64B5F6"),
		},
		Operator: ansi.StylePrimitive{
			Color: stringPtr("#4DD0E1"),
		},
		Punctuation: ansi.StylePrimitive{
			Color: stringPtr("#90A4AE"),
		},
		NameBuiltin: ansi.StylePrimitive{
			Color: stringPtr("#4DD0E1"),
		},
		NameTag: ansi.StylePrimitive{
			Color: stringPtr("#BA68C8"),
		},
		NameAttribute: ansi.StylePrimitive{
			Color: stringPtr("#64B5F6"),
		},
		NameClass: ansi.StylePrimitive{
			Color: stringPtr("#64B5F6"),
			Bold:  boolPtr(true),
		},
		NameFunction: ansi.StylePrimitive{
			Color: stringPtr("#4DD0E1"),
			Bold:  boolPtr(true),
		},
		LiteralNumber: ansi.StylePrimitive{
			Color: stringPtr("#FFD54F"),
		},
		LiteralString: ansi.StylePrimitive{
			Color: stringPtr("#81C784"),
		},
		LiteralStringEscape: ansi.StylePrimitive{
			Color: stringPtr("#FFD54F"),
		},
		GenericDeleted: ansi.StylePrimitive{
			Color: stringPtr("#E57373"),
		},
		GenericInserted: ansi.StylePrimitive{
			Color: stringPtr("#81C784"),
		},
		GenericEmph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		GenericStrong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func uintPtr(value uint) *uint {
	return &value
}
