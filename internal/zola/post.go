// Package zola models the Markdown content files deghosting generates for Zola.
package zola

import (
	"path"
	"time"
)

// Post is one generated Zola post.
type Post struct {
	Slug        string
	FrontMatter FrontMatter
	Body        string
	Images      []ImageRef
}

// BundleDir returns the post's bundle directory relative to the content root.
func (p Post) BundleDir() string {
	return p.Slug
}

// MarkdownPath returns the generated Markdown file path relative to the content
// root.
func (p Post) MarkdownPath() string {
	return path.Join(p.BundleDir(), "index.md")
}

// FrontMatter is the Zola TOML front matter for a generated post.
type FrontMatter struct {
	Title       string
	Date        time.Time
	Description string
	Authors     []string
	Taxonomies  Taxonomies
	Extra       Extra
}

// Taxonomies groups Zola taxonomy assignments.
type Taxonomies struct {
	Tags []string
}

// Extra holds theme-specific Zola front matter.
type Extra struct {
	FeatureImage string
}

// ImageRef is an unresolved image reference found while modeling a post.
type ImageRef struct {
	Source string
	Kind   ImageKind
}

// ImageKind identifies where an image reference came from in a Ghost post.
type ImageKind int

const (
	ImageKindUnknown ImageKind = iota
	ImageKindFeature
	ImageKindBody
	ImageKindOpenGraph
	ImageKindTwitter
)
