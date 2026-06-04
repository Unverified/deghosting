package deghosting

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Unverified/deghosting/internal/zola"
)

func writePosts(root string, posts []zola.Post) error {
	for _, post := range posts {
		err := writePost(root, &post)
		if err != nil {
			return err
		}
	}

	return nil
}

func writePost(root string, post *zola.Post) error {
	target := filepath.Join(root, filepath.FromSlash(post.MarkdownPath()))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create post bundle for %q: %w", post.Slug, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".index.md-*")
	if err != nil {
		return fmt.Errorf("create temporary markdown file for %q: %w", post.Slug, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if err = post.Markdown(tmp); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary markdown file for %q: %w", post.Slug, err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temporary markdown file for %q: %w", post.Slug, err)
	}

	if err = os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("write markdown file for %q: %w", post.Slug, err)
	}

	return nil
}
