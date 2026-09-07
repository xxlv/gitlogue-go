package highlighter

// Pool hands out one Highlighter per path so lexer/theme setup is paid once
// per file in a commit, not once per frame.
type Pool struct {
	theme string
	by    map[string]*Highlighter
}

// NewPool builds a pool for theme (empty means DefaultTheme).
func NewPool(theme string) *Pool {
	if theme == "" {
		theme = DefaultTheme
	}
	return &Pool{theme: theme, by: make(map[string]*Highlighter)}
}

// Theme is the Chroma style name this pool was created with.
func (p *Pool) Theme() string {
	if p == nil {
		return DefaultTheme
	}
	return p.theme
}

// For returns the highlighter bound to path, creating it on first use.
func (p *Pool) For(path string) *Highlighter {
	if p == nil {
		return New(path, DefaultTheme)
	}
	if h, ok := p.by[path]; ok {
		return h
	}
	h := New(path, p.theme)
	p.by[path] = h
	return h
}
