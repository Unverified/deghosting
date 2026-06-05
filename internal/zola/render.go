package zola

import (
	"fmt"
	"io"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Markdown renders p as a Zola Markdown file with TOML front matter.
func (p Post) Markdown(w io.Writer) error {
	if _, err := io.WriteString(w, "+++\n"); err != nil {
		return fmt.Errorf("write zola front matter delimiter for %q: %w", p.Slug, err)
	}
	if err := toml.NewEncoder(w).Encode(p.FrontMatter); err != nil {
		return fmt.Errorf("encode zola front matter for %q: %w", p.Slug, err)
	}
	if _, err := io.WriteString(w, "+++\n"); err != nil {
		return fmt.Errorf("write zola front matter delimiter for %q: %w", p.Slug, err)
	}

	body := strings.TrimSpace(p.Body)
	if body != "" {
		if _, err := io.WriteString(w, "\n"+body+"\n"); err != nil {
			return fmt.Errorf("write zola body for %q: %w", p.Slug, err)
		}
	}

	return nil
}
