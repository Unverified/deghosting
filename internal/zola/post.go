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
	Assets      []ImageAsset
}

// ImageAsset is a Ghost-hosted image reference resolved to a remote URL and the
// local path it is rewritten to, relative to the post bundle.
type ImageAsset struct {
	RemoteURL string
	LocalPath string // e.g. "b58f0c2e-photo.jpg", relative to index.md
	Kind      ImageKind
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
	Title       string     `toml:"title"`
	Date        time.Time  `toml:"date"`
	Description string     `toml:"description,omitempty"`
	Authors     []string   `toml:"authors,omitempty"`
	Taxonomies  Taxonomies `toml:"taxonomies,omitempty"`
	Extra       Extra      `toml:"extra,omitempty"`
}

// Taxonomies groups Zola taxonomy assignments.
type Taxonomies struct {
	Tags []string `toml:"tags,omitempty"`
}

// IsZero reports whether t contains no taxonomy assignments.
func (t Taxonomies) IsZero() bool {
	return len(t.Tags) == 0
}

// Extra holds theme-specific Zola front matter.
type Extra struct {
	FeatureImage string `toml:"feature_image,omitempty"`
	OGImage      string `toml:"og_image,omitempty"`
	TwitterImage string `toml:"twitter_image,omitempty"`
}

// IsZero reports whether e contains no theme-specific front matter.
func (e Extra) IsZero() bool {
	return e.FeatureImage == "" && e.OGImage == "" && e.TwitterImage == ""
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
