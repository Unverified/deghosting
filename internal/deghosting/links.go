package deghosting

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func rewriteBodyLinks(
	postSlug string,
	bodyHTML string,
	base *url.URL,
	convertedSlugs map[string]struct{},
) (string, []Warning) {
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, _ := html.ParseFragment(strings.NewReader(bodyHTML), ctx)

	var warnings []Warning
	warned := make(map[string]struct{})
	for _, n := range nodes {
		rewriteBodyLinkNode(n, postSlug, base, convertedSlugs, warned, &warnings)
	}

	var sb strings.Builder
	for _, n := range nodes {
		_ = html.Render(&sb, n)
	}

	return sb.String(), warnings
}

func rewriteBodyLinkNode(
	node *html.Node,
	postSlug string,
	base *url.URL,
	convertedSlugs map[string]struct{},
	warned map[string]struct{},
	warnings *[]Warning,
) {
	if node.Type == html.ElementNode && node.Data == "a" {
		rewriteBodyLinkHref(node, postSlug, base, convertedSlugs, warned, warnings)
	}
	for child := range node.ChildNodes() {
		rewriteBodyLinkNode(child, postSlug, base, convertedSlugs, warned, warnings)
	}
}

func rewriteBodyLinkHref(
	node *html.Node,
	postSlug string,
	base *url.URL,
	convertedSlugs map[string]struct{},
	warned map[string]struct{},
	warnings *[]Warning,
) {
	for i, attr := range node.Attr {
		if attr.Key != "href" {
			continue
		}
		target, isGhostHosted := internalPostLinkTarget(attr.Val, base, convertedSlugs)
		if !isGhostHosted {
			return
		}
		if target == "" {
			warnUnmatchedBodyLink(postSlug, attr.Val, warned, warnings)
			return
		}
		node.Attr[i].Val = target
		return
	}
}

func internalPostLinkTarget(
	href string,
	base *url.URL,
	convertedSlugs map[string]struct{},
) (target string, isGhostHosted bool) {
	ref, isGhostHosted := parseGhostHostedBodyLink(href, base)
	if !isGhostHosted {
		return "", false
	}

	slug, err := slugFromLinkPath(ref)
	if err != nil {
		return "", true
	}
	if _, ok := convertedSlugs[slug]; !ok {
		return "", true
	}

	target = fmt.Sprintf("@/%s/index.md", slug)
	if ref.Fragment != "" {
		target += "#" + ref.Fragment
	}
	return target, true
}

func parseGhostHostedBodyLink(href string, base *url.URL) (*url.URL, bool) {
	if strings.HasPrefix(href, ghostURLPlaceholder) {
		ref, err := url.Parse(href[len(ghostURLPlaceholder):])
		if err != nil {
			return &url.URL{}, true
		}
		return ref, true
	}

	ref, err := url.Parse(href)
	if err != nil {
		return &url.URL{}, false
	}
	if isSiteRelativeRef(href, ref) {
		return ref, true
	}
	if isHTTPURL(ref) && base != nil && sameOrigin(ref, base) {
		return ref, true
	}
	return nil, false
}

func slugFromLinkPath(ref *url.URL) (string, error) {
	linkPath, err := url.PathUnescape(ref.EscapedPath())
	if err != nil {
		return "", err
	}
	slug := strings.Trim(linkPath, "/")
	if slug == "" {
		return "", nil
	}
	return slug, nil
}

func warnUnmatchedBodyLink(
	postSlug string,
	href string,
	warned map[string]struct{},
	warnings *[]Warning,
) {
	if _, dup := warned[href]; dup {
		return
	}
	warned[href] = struct{}{}
	*warnings = append(*warnings, Warning{
		PostSlug: postSlug,
		Message:  fmt.Sprintf("Ghost-hosted body link %s does not match a converted post", href),
	})
}
