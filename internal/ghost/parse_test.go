package ghost_test

import (
	"strings"
	"testing"

	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/stretchr/testify/require"
)

// sampleExport mirrors the shapes seen in a real Ghost export: a published post
// with full fields, a draft with null published_at and null custom_excerpt, tag
// and author join tables, and a posts_meta table that does not cover every post.
const sampleExport = `{
  "db": [
    {
      "meta": {"exported_on": 1778780923254, "version": "5.95.0"},
      "data": {
        "posts": [
          {
            "id": "p1",
            "slug": "hello-world",
            "title": "Hello World",
            "html": "<p>hi</p>",
            "custom_excerpt": "A greeting.",
            "feature_image": "__GHOST_URL__/img.png",
            "og_image": "__GHOST_URL__/og.png",
            "twitter_image": "__GHOST_URL__/twitter.png",
            "status": "published",
            "type": "post",
            "visibility": "public",
            "published_at": "2020-05-19 12:03:00"
          },
          {
            "id": "p2",
            "slug": "untitled",
            "title": "Untitled",
            "html": "",
            "custom_excerpt": null,
            "feature_image": null,
            "og_image": null,
            "twitter_image": null,
            "status": "draft",
            "type": "post",
            "visibility": "public",
            "published_at": null
          }
        ],
        "tags": [
          {"id": "t1", "name": "Announcements", "slug": "announcements"},
          {"id": "t2", "name": "Adopters", "slug": "adopters"}
        ],
        "users": [
          {"id": "u1", "name": "Ada Lovelace", "slug": "ada", "email": "ada@example.com"},
          {"id": "u2", "name": "Grace Hopper", "slug": "grace", "email": "grace@example.com"}
        ],
        "posts_tags": [
          {"post_id": "p1", "tag_id": "t2", "sort_order": 1},
          {"post_id": "p1", "tag_id": "t1", "sort_order": 0}
        ],
        "posts_authors": [
          {"post_id": "p1", "author_id": "u1", "sort_order": 0},
          {"post_id": "p1", "author_id": "u2", "sort_order": 1}
        ],
        "posts_meta": [
          {"post_id": "p1", "meta_description": "SEO blurb."}
        ]
      }
    }
  ]
}`

func TestParse(t *testing.T) {
	t.Parallel()

	export, err := ghost.Parse(strings.NewReader(sampleExport))
	require.NoError(t, err)
	require.Len(t, export.DB, 1)

	db := export.DB[0]
	require.Equal(t, "5.95.0", db.Meta.Version)
	require.Len(t, db.Data.Posts, 2)
	require.Len(t, db.Data.Tags, 2)
	require.Len(t, db.Data.Users, 2)
	require.Len(t, db.Data.PostsTags, 2)
	require.Len(t, db.Data.PostsAuthors, 2)
	require.Len(t, db.Data.PostsMeta, 1, "posts_meta is sparse: not every post has a row")
}

func TestParsePublishedPost(t *testing.T) {
	t.Parallel()

	export, err := ghost.Parse(strings.NewReader(sampleExport))
	require.NoError(t, err)

	p := export.DB[0].Data.Posts[0]
	require.Equal(t, "hello-world", p.Slug)
	require.Equal(t, "Hello World", p.Title)
	require.Equal(t, "<p>hi</p>", p.HTML)
	require.Equal(t, "A greeting.", p.CustomExcerpt)
	require.Equal(t, "__GHOST_URL__/img.png", p.FeatureImage)
	require.Equal(t, "__GHOST_URL__/og.png", p.OGImage)
	require.Equal(t, "__GHOST_URL__/twitter.png", p.TwitterImage)
	require.Equal(t, "published", p.Status)
	require.False(t, p.PublishedAt.IsZero())
	require.Equal(t, "2020-05-19", p.PublishedAt.Format("2006-01-02"))
}

func TestParseAuthors(t *testing.T) {
	t.Parallel()

	export, err := ghost.Parse(strings.NewReader(sampleExport))
	require.NoError(t, err)

	db := export.DB[0]
	require.Equal(t, "Ada Lovelace", db.Data.Users[0].Name)
	require.Equal(t, "ada@example.com", db.Data.Users[0].Email)
	require.Equal(t, ghost.PostAuthor{
		PostID:    "p1",
		AuthorID:  "u1",
		SortOrder: 0,
	}, db.Data.PostsAuthors[0])
}

func TestParseDraftNullsDecodeToZeroValues(t *testing.T) {
	t.Parallel()

	export, err := ghost.Parse(strings.NewReader(sampleExport))
	require.NoError(t, err)

	p := export.DB[0].Data.Posts[1]
	require.Equal(t, "draft", p.Status)
	require.Empty(t, p.CustomExcerpt, "null custom_excerpt decodes to empty string")
	require.Empty(t, p.FeatureImage, "null feature_image decodes to empty string")
	require.Empty(t, p.OGImage, "null og_image decodes to empty string")
	require.Empty(t, p.TwitterImage, "null twitter_image decodes to empty string")
	require.True(t, p.PublishedAt.IsZero(), "null published_at decodes to the zero Time")
}

func TestParseInvalidJSON(t *testing.T) {
	t.Parallel()

	_, err := ghost.Parse(strings.NewReader("{not json"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode ghost export")
}

func TestParseRequiresExactlyOneDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty db array",
			input: `{"db":[]}`,
		},
		{
			name:  "multiple db entries",
			input: `{"db":[{},{}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := ghost.Parse(strings.NewReader(tc.input))
			require.Error(t, err)
			require.Contains(t, err.Error(), "expected exactly one db entry")
		})
	}
}

func TestParseBadTimestampErrors(t *testing.T) {
	t.Parallel()

	const badTime = `{"db":[{"data":{"posts":[{"published_at":"19/05/2020"}]}}]}`

	_, err := ghost.Parse(strings.NewReader(badTime))
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse ghost time")
}
