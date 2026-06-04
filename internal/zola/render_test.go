package zola_test

import (
	"bytes"
	"errors"
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
	want := `+++
title = 'Hello "World"'
date = 2020-05-19T12:03:00Z
description = 'A greeting.'
authors = ['Ada Lovelace', 'Grace Hopper']

[taxonomies]
tags = ['Announcements', 'Adopters']

[extra]
feature_image = '__GHOST_URL__/feature.png'
+++

Hello **Ghost**
`
	require.Equal(t, want, got.String())
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
	want := `+++
title = 'Hello World'
date = 2020-05-19T12:03:00Z
+++

`
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
