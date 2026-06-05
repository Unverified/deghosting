package download

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const maxDrainBytes = 16 << 10 // cap on error-body drain to allow conn reuse

func (d *Downloader) fetchAsset(ctx context.Context, j job) error {
	if err := os.MkdirAll(filepath.Dir(j.destPath), 0o755); err != nil {
		return fmt.Errorf("create asset directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(j.destPath), ".asset-*")
	if err != nil {
		return fmt.Errorf("create temp asset: %w", err)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	err = d.downloadFile(ctx, tmp, j.remoteURL)
	if err != nil {
		_ = tmp.Close()
		return err
	}

	err = tmp.Sync()
	if err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp asset: %w", err)
	}

	err = tmp.Close()
	if err != nil {
		return fmt.Errorf("close temp asset: %w", err)
	}

	err = os.Rename(tmpName, j.destPath)
	if err != nil {
		return fmt.Errorf("install asset: %w", err)
	}

	return nil
}

func (d *Downloader) downloadFile(ctx context.Context, dst *os.File, remoteURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return err
	}

	resp, err := d.httpDoer.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	_, err = io.Copy(dst, resp.Body)
	return err
}
