# Download Post assets and rewrite references into the bundle

Deghosting fetches every **Ghost-hosted** asset a Post references — its Feature
image, Social images (`og_image`, `twitter_image`), body `<img>` tags, and body
audio/video media — into the Post's page bundle as a Downloaded asset, and
rewrites those references to point at the colocated file so the content no longer
depends on the live Ghost host. References to **external** (third-party) hosts
are left untouched.

## Decisions

- **Required `--ghost-url`, no export fallback.** Ghost's canonical site URL lives
  in its config file, not reliably in the export's data, so we take the value
  that replaces the `__GHOST_URL__` placeholder as a CLI flag. It is required
  only when a placeholder or site-relative reference is actually encountered.
- **Only Ghost-hosted references are localized.** A reference is Ghost-hosted when
  it is a `__GHOST_URL__` placeholder, a site-relative path, or an absolute URL
  whose origin equals the Ghost URL. Only these are downloaded and rewritten;
  references to other hosts (including a separate CDN domain) are left as-is.
  Body media collection is limited to native media-bearing attributes:
  `<img src>`, `<audio src>`, `<video src>`, `<video poster>`, `<source src>`,
  and `<track src>`. `srcset` is intentionally ignored; the target site can
  regenerate responsive variants from the primary image.
- **Ghost-hosted Body links become Zola Internal Post links when the target
  exists.** Body `<a href>` values that point to a converted Post slug are
  rewritten to Zola's `@/.../index.md` source-content link form so Zola resolves
  the final public URL at build time. `__GHOST_URL__` placeholders and
  site-relative links can be rewritten without `--ghost-url`; absolute same-origin
  links require `--ghost-url` to classify. Query strings are dropped and
  fragments are preserved. Links that do not match a converted Post are left
  unchanged and reported as one warning per Post and target.
- **Preserve body audio/video as raw HTML.** Markdown has no equivalent for
  `<audio>`, `<video>`, `<source>`, or `<track>`, so body audio/video embeds are
  kept as raw HTML in the Markdown body whether their sources are Ghost-hosted or
  external. Ghost-hosted references inside them are rewritten before rendering; external
  references remain untouched. A `<figure>` that contains audio/video is also
  preserved as raw HTML so captions and Ghost card classes survive. Ordinary body
  images continue to convert to Markdown images.
- **Offline-first rewrite, then download.** Each Ghost-hosted reference is resolved
  against the Ghost URL to a remote URL and a deterministic local path of the form
  `<hash>-<basename>` (hash = a short SHA-256 prefix of the resolved URL,
  basename taken from the URL path; fallback `<hash>-asset` when the URL has no
  usable basename). References are rewritten to those local paths inside the
  parsed HTML DOM *before* HTML→Markdown conversion (and in the front-matter
  extras), so conversion never blocks on the network. Downloading is a separate
  stage. The same resolved URL within one Post dedupes to one file.
- **Retry then warn; references stay local on failure.** Each download is
  attempted up to three times. A permanent failure is surfaced as a warning and
  the already-rewritten local reference stays dangling, rather than aborting the
  run or falling back to the remote URL. A single 404 must not fail a 200-Post
  export.
- **Social images become front-matter extras.** `og_image` and `twitter_image`
  are added to the Zola `Extra` front matter (alongside `feature_image`) so their
  downloads are referenced rather than orphaned.
- **Always re-download, overwriting.** The download stage does not skip assets
  already present in the bundle; every run fetches fresh.
- **`internal/download` package with a managed downloader.** The download stage
  lives in its own package, separate from `internal/deghosting`. The entry point
  is `download.New(ctx, doer, cfg)`, which returns a downloader with its fixed
  worker pool already started. The downloader owns worker concurrency. Callers
  submit work with `Submit(remoteURL, destPath)` (returns once a worker accepts
  the job, does not wait for completion), then call
  `Close() []download.Failure` which blocks until all downloads settle and
  returns the URL and destination path for each failed download. Returning the
  destination path keeps post attribution correct when the same remote URL
  appears in multiple posts. The workers capture `ctx` from `New` — context is
  not stored on the struct, and `Submit` does not take one.
- **Retries live in the HTTP layer.** Production wiring passes a
  `hashicorp/go-retryablehttp` standard client into the downloader. The
  downloader does not own retry policy; it performs one request through its
  injected HTTP doer, then handles temp-file installation once.
- **CLI-owned downloader construction.** `CLI` receives a process callback.
  After Kong parses `--asset-concurrency`, `Execute` builds the parsed
  `download.Config` and passes it into the process callback.
  Production wiring wraps `http.DefaultClient` with
  `download.NewRetryableHTTPClient`, then calls `download.New(ctx, doer, cfg)`;
  tests inject stubs.
- **`Operation` owns downloader use and close.** `Operation` has a
  `download.AssetDownloader` field. It submits post asset jobs, converts failed
  jobs into post-scoped warnings, and closes the downloader on every exit path so
  the worker pool cannot be left waiting if conversion or writing fails.
- **`--asset-concurrency` flag.** Named for assets (not images) to accommodate
  future media types beyond images. Default: `runtime.NumCPU()` concurrency (0
  in the flag means NumCPU).

## Considered options

- *Read the base URL from the export.* Rejected: the site URL is not reliably
  present in a Ghost export's data, so the fallback would often be dead code.
- *Graceful fallback to the remote URL on download failure.* Rejected: it would
  force HTML→Markdown conversion to run after downloads complete, coupling
  conversion to the network; a loud warning on a dangling local reference is a
  fair trade for keeping conversion offline-first.
- *Skip assets already downloaded.* Rejected in favour of always overwriting.
- *Download external (third-party) assets too, for a fully self-contained bundle.*
  Rejected: deghosting only takes ownership of assets the Ghost host served;
  third-party references are left pointing at their original hosts.
- *Store context on `Downloader` struct.* Rejected: context is captured by the
  worker goroutine closures in `New`, keeping the struct free of lifecycle state
  and making the dependency explicit when workers are launched.
- *Pass context to `Submit`.* Rejected: `Submit` only enqueues; it does not
  perform work. The context belongs where the requests are made, which is in the
  workers started by `New`.
