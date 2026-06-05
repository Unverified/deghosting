package deghosting

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Unverified/deghosting/internal/zola"
)

// Doer is the subset of *http.Client used for image downloads.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// DownloadConfig controls the download stage.
type DownloadConfig struct {
	Timeout     time.Duration
	Concurrency int
}

type downloadFunc func(root string, posts []zola.Post, d Doer, cfg DownloadConfig) []Warning

// downloadAssets fetches every Ghost-hosted image asset from each post into its
// bundle directory. Each asset is attempted up to 3 times; a permanent failure
// emits a Warning and leaves the previously rewritten local reference dangling.
// Assets are downloaded in parallel across a single worker pool sized by
// cfg.Concurrency.
func downloadAssets(root string, posts []zola.Post, d Doer, cfg DownloadConfig) []Warning {
	type job struct {
		slug  string
		asset zola.ImageAsset
	}

	var jobs []job
	for _, post := range posts {
		for _, asset := range post.Assets {
			jobs = append(jobs, job{post.Slug, asset})
		}
	}
	if len(jobs) == 0 {
		return nil
	}

	jobCh := make(chan job, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	resultCh := make(chan *Warning, len(jobs))

	workers := max(cfg.Concurrency, 1)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for j := range jobCh {
				resultCh <- fetchAsset(root, j.slug, j.asset, d, cfg.Timeout)
			}
		})
	}
	wg.Wait()
	close(resultCh)

	var warnings []Warning
	for w := range resultCh {
		if w != nil {
			warnings = append(warnings, *w)
		}
	}
	return warnings
}

// fetchAsset downloads one asset into root/<slug>/<LocalPath>, retrying up to
// 3 times. Returns a Warning on permanent failure, nil on success.
func fetchAsset(root, slug string, asset zola.ImageAsset, d Doer, timeout time.Duration) *Warning {
	target := filepath.Join(root, slug, asset.LocalPath)
	for range 3 {
		if err := tryFetch(target, asset.RemoteURL, d, timeout); err == nil {
			return nil
		}
	}
	return &Warning{
		PostSlug: slug,
		Message:  fmt.Sprintf("failed to download %s after 3 attempts", asset.RemoteURL),
	}
}

func tryFetch(target, remoteURL string, d Doer, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if err = os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".img-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err = io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, target)
}
