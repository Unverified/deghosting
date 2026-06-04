// Package ghost models the subset of a Ghost JSON export that deghosting needs
// to produce Markdown content. It captures only the fields that drive the
// generated output — Post (html, slug, title, custom_excerpt, feature_image,
// dates, status), Tag, the posts_tags join, and meta_description — and ignores
// the rest of the export (members, settings, Stripe, snippets, newsletters).
//
// See https://ghost.org/help/exports/ for how the input is produced and
// https://docs.ghost.org/admin-api/posts/overview for Post field semantics.
//
// # Export realities the parser handles
//
// These were verified against the example export:
//
//   - Timestamps are "2006-01-02 15:04:05" (space-separated UTC), not RFC 3339,
//     so [Time] implements the json/v2 [json.UnmarshalerFrom] interface. A JSON
//     null (drafts' published_at) decodes to the zero Time.
//   - custom_excerpt and feature_image are null on some posts, decoding to "".
//   - posts_meta is sparse (449 rows for 518 posts); the tag and meta joins must
//     tolerate a missing row.
//   - status is trustworthy: filtering published vs draft matches what is live.
//
// # Deferred to transformation, not parsing
//
// The following are intentionally out of scope here and belong to the (not yet
// built) conversion step:
//
//   - The __GHOST_URL__ placeholder appears in feature_image and inside post HTML
//     and must be rewritten to real paths.
//   - description can be genuinely empty: a few live posts have neither a
//     custom_excerpt nor a meta_description row, so the fallback chain bottoms out
//     and a third fallback (or omission) is needed.
package ghost
