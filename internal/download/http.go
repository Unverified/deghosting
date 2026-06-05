package download

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	retryMax     = 2 // retryablehttp counts retries after the first attempt
	retryWaitMin = 100 * time.Millisecond
	retryWaitMax = time.Second
)

// NewRetryableHTTPClient returns a standard-library HTTP client whose transport
// retries transient request failures and retryable HTTP statuses.
func NewRetryableHTTPClient(base *http.Client) *http.Client {
	retryClient := retryablehttp.NewClient()
	retryClient.Logger = nil
	retryClient.RetryMax = retryMax
	retryClient.RetryWaitMin = retryWaitMin
	retryClient.RetryWaitMax = retryWaitMax

	httpClient := retryClient.HTTPClient
	if base != nil {
		httpClient = new(*base)
	}
	retryClient.HTTPClient = httpClient

	return retryClient.StandardClient()
}
