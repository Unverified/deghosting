# Download Post images and rewrite references into the bundle

Deghosting fetches every **Ghost-hosted** image a Post references — its Feature
image, Social images (`og_image`, `twitter_image`), and body `<img>` tags — into
the Post's page bundle as a Downloaded asset, and rewrites those references to
point at the colocated file so the content no longer depends on the live Ghost
host. References to **external** (third-party) hosts are left untouched.

## Decisions

- **Required `--ghost-url`, no export fallback.** Ghost's canonical site URL lives
  in its config file, not reliably in the export's data, so we take the value
  that replaces the `__GHOST_URL__` placeholder as a CLI flag. It is required
  only when a placeholder or site-relative reference is actually encountered.
- **Only Ghost-hosted references are localized.** A reference is Ghost-hosted when
  it is a `__GHOST_URL__` placeholder, a site-relative path, or an absolute URL
  whose origin equals the Ghost URL. Only these are downloaded and rewritten;
  references to other hosts (including a separate CDN domain) are left as-is.
- **Offline-first rewrite, then download.** Each Ghost-hosted reference is resolved
  against the Ghost URL to a remote URL and a deterministic local path of the form
  `<hash>-<basename>` (hash = a short SHA-256 prefix of the resolved URL,
  extension taken from the URL). References are rewritten to those local paths
  inside the parsed HTML DOM *before* HTML→Markdown conversion (and in the
  front-matter extras), so conversion never blocks on the network. Downloading is
  a separate stage. The same resolved URL within one Post dedupes to one file.
- **Retry then warn; references stay local on failure.** Each download is
  attempted up to three times. A permanent failure emits a `Warning` and leaves
  the already-rewritten local reference dangling, rather than aborting the run or
  falling back to the remote URL. A single 404 must not fail a 200-Post export.
- **Social images become front-matter extras.** `og_image` and `twitter_image`
  are added to the Zola `Extra` front matter (alongside `feature_image`) so their
  downloads are referenced rather than orphaned.
- **Always re-download, overwriting.** The download stage does not skip assets
  already present in the bundle; every run fetches fresh.
- **Injected fetcher.** Downloading runs through an injected HTTP doer (default
  `http.DefaultClient`) behind a per-request timeout, mirroring the existing
  `convertFunc`/`writePostsFunc` seams on `Operation`, so tests never touch the
  network. Concurrency is a single global worker pool. `--image-timeout`
  (default 10s) and `--image-concurrency` (default `runtime.NumCPU()`) tune it.

## Considered options

- *Read the base URL from the export.* Rejected: the site URL is not reliably
  present in a Ghost export's data, so the fallback would often be dead code.
- *Graceful fallback to the remote URL on download failure.* Rejected: it would
  force HTML→Markdown conversion to run after downloads complete, coupling
  conversion to the network; a loud warning on a dangling local reference is a
  fair trade for keeping conversion offline-first.
- *Skip assets already downloaded.* Rejected in favor of always overwriting.
- *Download external (third-party) images too, for a fully self-contained bundle.*
  Rejected: deghosting only takes ownership of assets the Ghost host served;
  third-party references are left pointing at their original hosts.
