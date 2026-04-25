package markdown

import (
	"github.com/charmbracelet/glamour"
	repltheme "github.com/user/keen-code/internal/cli/repl/theme"
)

type Renderer struct {
	renderer *glamour.TermRenderer
	width    int
}

func newGlamourRenderer(width int) (*glamour.TermRenderer, error) {
	return glamour.NewTermRenderer(
		glamour.WithStyles(repltheme.MarkdownStyleConfig()),
		glamour.WithChromaFormatter("terminal256"),
		glamour.WithWordWrap(width-4),
	)
}

func New(width int) (*Renderer, error) {
	renderer, err := newGlamourRenderer(width)
	if err != nil {
		return nil, err
	}

	return &Renderer{
		renderer: renderer,
		width:    width,
	}, nil
}

func (r *Renderer) Render(markdown string) string {
	if markdown == "" {
		return ""
	}

	rendered, err := r.renderer.Render(markdown)
	if err != nil {
		return markdown
	}
	return rendered
}

func (r *Renderer) UpdateWidth(width int) error {
	if r.width == width {
		return nil
	}

	renderer, err := newGlamourRenderer(width)
	if err != nil {
		return err
	}

	r.renderer = renderer
	r.width = width
	return nil
}
