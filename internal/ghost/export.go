package ghost

import (
	"encoding/json/v2"
	"fmt"
	"io"
)

// Export is the root object of a Ghost export file. Ghost wraps everything in a
// single-element "db" array; the meaningful payload lives in DB[0].
type Export struct {
	DB []Database `json:"db"`
}

// Database is one snapshot inside an export: its table data plus export metadata.
type Database struct {
	Meta Meta `json:"meta"`
	Data Data `json:"data"`
}

// Meta records when the export was produced and which Ghost version wrote it.
type Meta struct {
	// ExportedOn is a Unix timestamp in milliseconds.
	ExportedOn int64  `json:"exported_on"`
	Version    string `json:"version"`
}

// Data holds the exported tables. Ghost stores many-to-many relationships (such
// as a post's tags and authors) in separate join tables, so resolving a post's
// related records means joining by ID.
type Data struct {
	Posts        []Post       `json:"posts"`
	Tags         []Tag        `json:"tags"`
	Users        []User       `json:"users"`
	PostsTags    []PostTag    `json:"posts_tags"`
	PostsAuthors []PostAuthor `json:"posts_authors"`
	PostsMeta    []PostMeta   `json:"posts_meta"`
}

// Post is a single blog entry. The rendered article lives in HTML; the other
// fields supply the Markdown front matter.
type Post struct {
	ID   string `json:"id"`
	UUID string `json:"uuid"`

	// Slug is the URL path of the post and the basis for the output filename.
	Slug  string `json:"slug"`
	Title string `json:"title"`

	// HTML is the rendered post body — the primary content to convert.
	HTML string `json:"html"`

	// CustomExcerpt is the post's handwritten summary. It is null for drafts and
	// any post without a custom excerpt, decoding to "".
	CustomExcerpt string `json:"custom_excerpt"`

	// FeatureImage is the hero image URL; null (→ "") when unset. Ghost rewrites
	// site-relative URLs to a "__GHOST_URL__" placeholder.
	FeatureImage string `json:"feature_image"`
	// OGImage is the Open Graph social preview image URL; null (→ "") when unset.
	OGImage string `json:"og_image"`
	// TwitterImage is the Twitter/X social preview image URL; null (→ "") when unset.
	TwitterImage string `json:"twitter_image"`

	// Status is "published" or "draft"; Type is "post" or "page".
	Status string `json:"status"`
	Type   string `json:"type"`
	// Visibility is "public", "members", "paid", etc.
	Visibility string `json:"visibility"`

	// PublishedAt is null (→ zero Time) for unpublished drafts.
	PublishedAt Time `json:"published_at"`
	UpdatedAt   Time `json:"updated_at"`
	CreatedAt   Time `json:"created_at"`
}

// Tag is a label that posts can be filed under. Posts reference tags through the
// PostsTags join table rather than embedding them.
type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// User is a Ghost staff user who may be credited as a post author.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Slug  string `json:"slug"`
	Email string `json:"email"`
}

// PostTag joins a post to a tag. SortOrder preserves the author's tag ordering;
// the first tag (SortOrder 0) is Ghost's "primary tag".
type PostTag struct {
	PostID    string `json:"post_id"`
	TagID     string `json:"tag_id"`
	SortOrder int    `json:"sort_order"`
}

// PostAuthor joins a post to a user credited as an author.
type PostAuthor struct {
	PostID    string `json:"post_id"`
	AuthorID  string `json:"author_id"`
	SortOrder int    `json:"sort_order"`
}

// PostMeta carries a post's SEO and social metadata. Only MetaDescription is
// modeled here, as a fallback source for the front-matter description when a
// post has no custom excerpt.
type PostMeta struct {
	PostID          string `json:"post_id"`
	MetaDescription string `json:"meta_description"`
}

// Parse reads a Ghost JSON export from r and decodes it into an Export. It does
// not interpret or transform the content; callers get the raw export structure
// to work with. A malformed-but-valid export (missing fields, null values, or
// sparse join tables) decodes successfully, with absent data left as zero values.
func Parse(r io.Reader) (*Export, error) {
	var export Export
	if err := json.UnmarshalRead(r, &export); err != nil {
		return nil, fmt.Errorf("decode ghost export: %w", err)
	}
	if len(export.DB) != 1 {
		return nil, fmt.Errorf("decode ghost export: expected exactly one db entry, got %d", len(export.DB))
	}

	return &export, nil
}
