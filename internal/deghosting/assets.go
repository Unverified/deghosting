package deghosting

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/Unverified/deghosting/internal/ghost"
	"github.com/Unverified/deghosting/internal/zola"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const ghostURLPlaceholder = "__GHOST_URL__"

type assetCandidate struct {
	src  string
	kind zola.AssetKind
}

// processAssets resolves Ghost-hosted asset references in post, rewrites body
// HTML in a single DOM parse, and returns the rewritten HTML and planned assets.
// base is the parsed Ghost URL; pass nil when --ghost-url was not provided. If a
// Ghost-hosted reference is encountered with nil base, an error naming the post
// and reference is returned. Assets are deduplicated by resolved remote URL;
// order is first-seen stable.
func processAssets(post ghost.Post, base *url.URL) (rewrittenHTML string, assets []zola.Asset, err error) {
	// Parse the body HTML once. We collect references, rewrite, and render from the same DOM.
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"}
	nodes, _ := html.ParseFragment(strings.NewReader(post.HTML), ctx)

	candidates := []assetCandidate{
		{src: post.FeatureImage, kind: zola.AssetKindFeatureImage},
		{src: post.OGImage, kind: zola.AssetKindOpenGraphImage},
		{src: post.TwitterImage, kind: zola.AssetKindTwitterImage},
	}
	for _, n := range nodes {
		collectAssetCandidates(n, false, &candidates)
	}

	// Validate upfront: Ghost-hosted refs require a base URL.
	if base == nil {
		for _, c := range candidates {
			if c.src == "" {
				continue
			}
			if _, isGhost := resolveGhostRef(c.src, nil); isGhost {
				return "", nil, fmt.Errorf("process assets for %q: Ghost-hosted reference %q requires --ghost-url", post.Slug, c.src)
			}
		}
	}

	// Resolve Ghost-hosted candidates into assets and a src-to-local-path lookup.
	seen := make(map[string]struct{})
	remoteToLocal := make(map[string]string)
	for _, c := range candidates {
		if c.src == "" {
			continue
		}
		remote, isGhost := resolveGhostRef(c.src, base)
		if !isGhost {
			continue
		}
		if _, dup := seen[remote]; dup {
			continue
		}
		seen[remote] = struct{}{}
		lp := localPath(remote)
		remoteToLocal[remote] = lp
		assets = append(assets, zola.Asset{
			RemoteURL: remote,
			LocalPath: lp,
			Kind:      c.kind,
		})
	}

	// Rewrite Ghost-hosted body asset references to local paths in the already-parsed DOM.
	for _, n := range nodes {
		rewriteAssetRefs(n, false, base, remoteToLocal)
	}

	// Render the DOM once.
	var sb strings.Builder
	for _, n := range nodes {
		if err := html.Render(&sb, n); err != nil {
			return "", nil, fmt.Errorf("render rewritten HTML for %q: %w", post.Slug, err)
		}
	}

	return sb.String(), assets, nil
}

// lookupLocal returns the planned local path for src when it is a Ghost-hosted
// reference present in remoteToLocal, otherwise returns src unchanged.
func lookupLocal(src string, base *url.URL, remoteToLocal map[string]string) string {
	if src == "" {
		return ""
	}
	remote, isGhost := resolveGhostRef(src, base)
	if !isGhost {
		return src
	}
	if local, ok := remoteToLocal[remote]; ok {
		return local
	}
	return src
}

// resolveGhostRef classifies src and, if Ghost-hosted, returns its resolved
// remote URL. For placeholder and site-relative srcs, a nil base still reports
// isGhost so callers can require --ghost-url without risking a panic.
func resolveGhostRef(src string, base *url.URL) (remote string, isGhost bool) {
	if strings.HasPrefix(src, ghostURLPlaceholder) {
		if base == nil {
			return "", true
		}
		return base.String() + src[len(ghostURLPlaceholder):], true
	}

	ref, err := url.Parse(src)
	if err != nil {
		return "", false
	}
	if isSiteRelativeRef(src, ref) {
		if base == nil {
			return "", true
		}
		return base.ResolveReference(ref).String(), true
	}

	if !isHTTPURL(ref) {
		return "", false
	}
	if base != nil && sameOrigin(ref, base) {
		return ref.String(), true
	}
	return "", false
}

func parseGhostOrigin(raw string) (*url.URL, error) {
	origin, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !isHTTPURL(origin) {
		return nil, fmt.Errorf("must be an http or https origin")
	}
	if origin.User != nil {
		return nil, fmt.Errorf("must not include user info")
	}
	if origin.Path != "" && origin.Path != "/" {
		return nil, fmt.Errorf("must not include a path")
	}
	if origin.RawQuery != "" {
		return nil, fmt.Errorf("must not include a query")
	}
	if origin.Fragment != "" {
		return nil, fmt.Errorf("must not include a fragment")
	}

	origin.Path = ""
	origin.RawPath = ""
	origin.ForceQuery = false
	return origin, nil
}

func isSiteRelativeRef(src string, ref *url.URL) bool {
	return strings.HasPrefix(src, "/") &&
		!strings.HasPrefix(src, "//") &&
		ref.Scheme == "" &&
		ref.Host == ""
}

func isHTTPURL(ref *url.URL) bool {
	return ref.Host != "" && (ref.Scheme == "http" || ref.Scheme == "https")
}

func sameOrigin(ref, base *url.URL) bool {
	return ref.Scheme == base.Scheme && strings.EqualFold(ref.Host, base.Host)
}

// localPath computes the deterministic bundle-relative filename for remoteURL:
// "<hash>-<basename>" where hash is the first 8 hex chars of sha256(remoteURL).
func localPath(remoteURL string) string {
	sum := sha256.Sum256([]byte(remoteURL))
	hash := fmt.Sprintf("%x", sum[:4]) // 4 bytes = 8 hex chars
	u, err := url.Parse(remoteURL)
	if err != nil {
		return fmt.Sprintf("%s-asset", hash)
	}
	base := path.Base(u.Path)
	if base == "." || base == "/" {
		return fmt.Sprintf("%s-asset", hash)
	}
	return fmt.Sprintf("%s-%s", hash, base)
}

func collectAssetCandidates(
	node *html.Node,
	insideMedia bool,
	candidates *[]assetCandidate,
) {
	if node.Type == html.ElementNode {
		switch node.Data {
		case "img":
			if src := htmlAttr(node, "src"); src != "" {
				*candidates = append(*candidates, assetCandidate{src: src, kind: zola.AssetKindBodyImage})
			}
		case "audio":
			insideMedia = true
			if src := htmlAttr(node, "src"); src != "" {
				*candidates = append(*candidates, assetCandidate{src: src, kind: zola.AssetKindBodyAudio})
			}
		case "video":
			insideMedia = true
			if src := htmlAttr(node, "src"); src != "" {
				*candidates = append(*candidates, assetCandidate{src: src, kind: zola.AssetKindBodyVideo})
			}
			if poster := htmlAttr(node, "poster"); poster != "" {
				*candidates = append(*candidates, assetCandidate{src: poster, kind: zola.AssetKindBodyVideoPoster})
			}
		case "source":
			if insideMedia {
				if src := htmlAttr(node, "src"); src != "" {
					*candidates = append(*candidates, assetCandidate{src: src, kind: zola.AssetKindBodyMediaSource})
				}
			}
		case "track":
			if insideMedia {
				if src := htmlAttr(node, "src"); src != "" {
					*candidates = append(*candidates, assetCandidate{src: src, kind: zola.AssetKindBodyMediaTrack})
				}
			}
		}
	}
	for child := range node.ChildNodes() {
		collectAssetCandidates(child, insideMedia, candidates)
	}
}

func rewriteAssetRefs(
	node *html.Node,
	insideMedia bool,
	base *url.URL,
	remoteToLocal map[string]string,
) {
	if node.Type == html.ElementNode {
		switch node.Data {
		case "img":
			rewriteAssetAttr(node, "src", base, remoteToLocal)
		case "audio":
			insideMedia = true
			rewriteAssetAttr(node, "src", base, remoteToLocal)
		case "video":
			insideMedia = true
			rewriteAssetAttr(node, "src", base, remoteToLocal)
			rewriteAssetAttr(node, "poster", base, remoteToLocal)
		case "source":
			if insideMedia {
				rewriteAssetAttr(node, "src", base, remoteToLocal)
			}
		case "track":
			if insideMedia {
				rewriteAssetAttr(node, "src", base, remoteToLocal)
			}
		}
	}
	for child := range node.ChildNodes() {
		rewriteAssetRefs(child, insideMedia, base, remoteToLocal)
	}
}

func rewriteAssetAttr(
	node *html.Node,
	key string,
	base *url.URL,
	remoteToLocal map[string]string,
) {
	for i, attr := range node.Attr {
		if attr.Key != key {
			continue
		}
		if local := lookupLocal(attr.Val, base, remoteToLocal); local != attr.Val {
			node.Attr[i].Val = local
		}
		return
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
