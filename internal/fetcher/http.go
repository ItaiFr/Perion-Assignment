package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"Perion_Assignment/internal/models"
)

// HTTPFetcher implements Service using HTTP requests
type HTTPFetcher struct {
	client  *http.Client
	timeout time.Duration
}

// NewHTTPFetcher creates a new HTTP-based ads.txt fetcher
func NewHTTPFetcher(timeout time.Duration) Service {
	return newHTTPFetcher(timeout)
}

// newHTTPFetcher creates the concrete implementation
func newHTTPFetcher(timeout time.Duration) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 5 redirects
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		timeout: timeout,
	}
}

// Fetch retrieves the ads.txt file for the given domain
func (f *HTTPFetcher) Fetch(ctx context.Context, domain string) (string, error) {
	if domain == "" {
		return "", models.ErrInvalidDomain
	}

	// Normalize domain
	normalizedDomain := f.normalizeDomain(domain)
	
	// Construct ads.txt URL
	adsTxtURL := fmt.Sprintf("https://%s/ads.txt", normalizedDomain)
	
	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adsTxtURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set appropriate headers
	req.Header.Set("User-Agent", "AdsTxt-Analyzer/1.0")
	req.Header.Set("Accept", "text/plain")
	
	// Perform request
	resp, err := f.client.Do(req)
	if err != nil {
		// Check for timeout
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("%w: %v", models.ErrFetchTimeout, err)
		}
		return "", fmt.Errorf("failed to fetch ads.txt: %w", err)
	}
	defer resp.Body.Close()
	
	// Check response status
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("%w: HTTP %d", models.ErrDomainNotFound, resp.StatusCode)
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected HTTP status: %d %s", resp.StatusCode, resp.Status)
	}
	
	// Read response body with size limit
	body, err := f.readBodyWithLimit(resp.Body, 1024*1024) // 1MB limit
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	
	return string(body), nil
}

// normalizeDomain removes protocol, port, and path from domain
func (f *HTTPFetcher) normalizeDomain(domain string) string {
	// Remove protocol if present
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	
	// Parse to handle edge cases
	if !strings.Contains(domain, "/") {
		domain = "http://" + domain
	} else {
		domain = "http://" + domain
	}
	
	parsedURL, err := url.Parse(domain)
	if err != nil {
		// Fallback: just return the cleaned domain
		return strings.Split(strings.TrimPrefix(domain, "http://"), "/")[0]
	}
	
	return parsedURL.Hostname()
}

// readBodyWithLimit reads the response body with a size limit
func (f *HTTPFetcher) readBodyWithLimit(body io.Reader, maxSize int64) ([]byte, error) {
	limitedReader := io.LimitReader(body, maxSize)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	
	// Check if we hit the limit
	if int64(len(data)) >= maxSize {
		return nil, fmt.Errorf("ads.txt file too large (exceeds %d bytes)", maxSize)
	}
	
	return data, nil
}