package ghost

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
// as a post's tags) in separate join tables, so resolving a post's tags means
// joining Posts → PostsTags → Tags by ID.
type Data struct {
	Posts     []Post     `json:"posts"`
	Tags      []Tag      `json:"tags"`
	PostsTags []PostTag  `json:"posts_tags"`
	PostsMeta []PostMeta `json:"posts_meta"`
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

	// CustomExcerpt is the post's hand-written summary. It is null for drafts and
	// any post without a custom excerpt, decoding to "".
	CustomExcerpt string `json:"custom_excerpt"`

	// FeatureImage is the hero image URL; null (→ "") when unset. Ghost rewrites
	// site-relative URLs to a "__GHOST_URL__" placeholder.
	FeatureImage string `json:"feature_image"`

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

// PostTag joins a post to a tag. SortOrder preserves the author's tag ordering;
// the first tag (SortOrder 0) is Ghost's "primary tag".
type PostTag struct {
	PostID    string `json:"post_id"`
	TagID     string `json:"tag_id"`
	SortOrder int    `json:"sort_order"`
}

// PostMeta carries a post's SEO and social metadata. Only MetaDescription is
// modeled here, as a fallback source for the front-matter description when a
// post has no custom excerpt.
type PostMeta struct {
	PostID          string `json:"post_id"`
	MetaDescription string `json:"meta_description"`
}
