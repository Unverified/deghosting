// Package download manages a pool of concurrent asset downloads.
package download

import (
	"context"
	"net/http"
	"sync"
)

// HTTPDoer is the subset of *http.Client used for downloads.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// Config controls asset downloads. Constructors replace zero values with sane
// defaults for the fields they use.
type Config struct {
	Concurrency int // worker count; <1 -> 1
}

// AssetDownloader submits download jobs and waits for the pool to finish.
type AssetDownloader interface {
	Submit(remoteURL, destPath string)
	Close() []Failure
}

// Failure describes a download that failed.
type Failure struct {
	RemoteURL string
	DestPath  string
	Err       error
}

type job struct {
	remoteURL string
	destPath  string
}

// Downloader manages a pool of concurrent download workers. New returns a
// started downloader; callers submit work and then Close it.
type Downloader struct {
	httpDoer  HTTPDoer
	jobs      chan job
	wg        sync.WaitGroup
	closeOnce sync.Once

	mu       sync.Mutex
	failures []Failure
}

// New constructs a Downloader, normalizes cfg, and starts the worker pool.
func New(ctx context.Context, doer HTTPDoer, cfg Config) *Downloader {
	cfg.Concurrency = max(cfg.Concurrency, 1)

	d := &Downloader{
		httpDoer: doer,
		jobs:     make(chan job, cfg.Concurrency),
	}
	for range cfg.Concurrency {
		d.wg.Go(func() { d.worker(ctx) })
	}
	return d
}

func (d *Downloader) worker(ctx context.Context) {
	for j := range d.jobs {
		d.process(ctx, j)
	}
}

func (d *Downloader) process(ctx context.Context, j job) {
	err := d.fetchAsset(ctx, j)

	if err != nil {
		fail := Failure{
			RemoteURL: j.remoteURL,
			DestPath:  j.destPath,
			Err:       err,
		}
		d.recordFailure(fail)
	}
}

func (d *Downloader) recordFailure(f Failure) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.failures = append(d.failures, f)
}

// Submit enqueues a download. It returns once the job is buffered or a worker
// accepts it; it does not wait for completion. Calling Submit after Close
// panics (send on closed channel) because the caller owns submission ordering.
func (d *Downloader) Submit(remoteURL, destPath string) {
	d.jobs <- job{remoteURL: remoteURL, destPath: destPath}
}

// Close signals no more jobs, blocks until in-flight downloads finish, and
// returns the failures. Safe to call more than once.
func (d *Downloader) Close() []Failure {
	d.closeOnce.Do(func() { close(d.jobs) })
	d.wg.Wait()
	return d.failures
}
