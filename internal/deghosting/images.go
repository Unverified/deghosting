package deghosting

import (
	"strings"

	"golang.org/x/net/html"

	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
)

func imageRefs(post ghost.Post) []zola.ImageRef {
	candidates := []zola.ImageRef{
		{Source: post.FeatureImage, Kind: zola.ImageKindFeature},
		{Source: post.OGImage, Kind: zola.ImageKindOpenGraph},
		{Source: post.TwitterImage, Kind: zola.ImageKindTwitter},
	}
	candidates = append(candidates, bodyImageRefs(post.HTML)...)

	refs := make([]zola.ImageRef, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Source == "" {
			continue
		}
		refs = append(refs, candidate)
	}

	return refs
}

func bodyImageRefs(body string) []zola.ImageRef {
	nodes, err := html.ParseFragment(strings.NewReader(body), nil)
	if err != nil {
		return nil
	}

	var refs []zola.ImageRef
	for _, node := range nodes {
		collectBodyImageRefs(node, &refs)
	}

	return refs
}

func collectBodyImageRefs(node *html.Node, refs *[]zola.ImageRef) {
	if node.Type == html.ElementNode && node.Data == "img" {
		if src := htmlAttr(node, "src"); src != "" {
			*refs = append(*refs, zola.ImageRef{
				Source: src,
				Kind:   zola.ImageKindBody,
			})
		}
	}

	for child := range node.ChildNodes() {
		collectBodyImageRefs(child, refs)
	}
}

func htmlAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
