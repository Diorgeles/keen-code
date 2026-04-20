package markdown

import "github.com/charmbracelet/glamour"

type Renderer struct {
	renderer *glamour.TermRenderer
	width    int
}

func New(width int) (*Renderer, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width-4),
	)
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

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return err
	}

	r.renderer = renderer
	r.width = width
	return nil
}
