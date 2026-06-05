package zola_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Unverified/deghosting/internal/zola"
	"github.com/stretchr/testify/require"
)

func TestPostMarkdown(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2020, 5, 19, 12, 3, 0, 0, time.UTC)
	post := zola.Post{
		Slug: "hello-world",
		FrontMatter: zola.FrontMatter{
			Title:       `Hello "World"`,
			Date:        publishedAt,
			Description: "A greeting.",
			Authors:     []string{"Ada Lovelace", "Grace Hopper"},
			Taxonomies: zola.Taxonomies{
				Tags: []string{"Announcements", "Adopters"},
			},
			Extra: zola.Extra{
				FeatureImage: "__GHOST_URL__/feature.png",
			},
		},
		Body: "Hello **Ghost**\n",
	}

	var got bytes.Buffer
	err := post.Markdown(&got)

	require.NoError(t, err)
	want := strings.Join([]string{
		"+++",
		`title = 'Hello "World"'`,
		"date = 2020-05-19T12:03:00Z",
		"description = 'A greeting.'",
		"authors = ['Ada Lovelace', 'Grace Hopper']",
		"",
		"[taxonomies]",
		"tags = ['Announcements', 'Adopters']",
		"",
		"[extra]",
		"feature_image = '__GHOST_URL__/feature.png'",
		"+++",
		"",
		"Hello **Ghost**",
		"",
	}, "\n")
	require.Equal(t, want, got.String())
}

func TestPostMarkdownSocialImageExtras(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		extra zola.Extra
		want  string
	}{
		{
			name:  "all three extras",
			extra: zola.Extra{FeatureImage: "a.jpg", OGImage: "b.jpg", TwitterImage: "c.jpg"},
			want: strings.Join([]string{
				"[extra]",
				"feature_image = 'a.jpg'",
				"og_image = 'b.jpg'",
				"twitter_image = 'c.jpg'",
				"",
			}, "\n"),
		},
		{
			name:  "og only",
			extra: zola.Extra{OGImage: "b.jpg"},
			want: strings.Join([]string{
				"[extra]",
				"og_image = 'b.jpg'",
				"",
			}, "\n"),
		},
		{
			name:  "twitter only",
			extra: zola.Extra{TwitterImage: "c.jpg"},
			want: strings.Join([]string{
				"[extra]",
				"twitter_image = 'c.jpg'",
				"",
			}, "\n"),
		},
		{
			name:  "none — no extra block",
			extra: zola.Extra{},
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			post := zola.Post{
				Slug: "test",
				FrontMatter: zola.FrontMatter{
					Title: "Test",
					Date:  time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Extra: tc.extra,
				},
			}

			var got bytes.Buffer
			require.NoError(t, post.Markdown(&got))

			if tc.want == "" {
				require.NotContains(t, got.String(), "[extra]")
			} else {
				require.Contains(t, got.String(), tc.want)
			}
		})
	}
}

func TestExtraIsZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		extra zola.Extra
		want  bool
	}{
		{"all empty", zola.Extra{}, true},
		{"feature only", zola.Extra{FeatureImage: "a.jpg"}, false},
		{"og only", zola.Extra{OGImage: "b.jpg"}, false},
		{"twitter only", zola.Extra{TwitterImage: "c.jpg"}, false},
		{"all set", zola.Extra{FeatureImage: "a.jpg", OGImage: "b.jpg", TwitterImage: "c.jpg"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.extra.IsZero())
		})
	}
}

func TestPostMarkdownOmitsEmptyOptionalFrontMatter(t *testing.T) {
	t.Parallel()

	post := zola.Post{
		Slug: "hello-world",
		FrontMatter: zola.FrontMatter{
			Title: "Hello World",
			Date:  time.Date(2020, 5, 19, 12, 3, 0, 0, time.UTC),
		},
	}

	var got bytes.Buffer
	err := post.Markdown(&got)

	require.NoError(t, err)
	want := strings.Join([]string{
		"+++",
		"title = 'Hello World'",
		"date = 2020-05-19T12:03:00Z",
		"+++",
		"",
	}, "\n")
	require.Equal(t, want, got.String())
}

func TestPostMarkdownReturnsWriterError(t *testing.T) {
	t.Parallel()

	post := zola.Post{
		Slug: "hello-world",
		FrontMatter: zola.FrontMatter{
			Title: "Hello World",
			Date:  time.Date(2020, 5, 19, 12, 3, 0, 0, time.UTC),
		},
	}
	writerErr := errors.New("write failed")

	err := post.Markdown(errorWriter{err: writerErr})

	require.ErrorIs(t, err, writerErr)
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}
