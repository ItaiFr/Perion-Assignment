package fetcher

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Perion_Assignment/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTransport wraps the default transport and rewrites URLs to use the test server
type testTransport struct {
	base      http.RoundTripper
	serverURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to use the test server
	// Replace the scheme, host, and port with the test server's
	req.URL.Scheme = "https"
	req.URL.Host = strings.TrimPrefix(t.serverURL, "https://")
	return t.base.RoundTrip(req)
}

// createTLSFetcher creates a fetcher that trusts the test server's certificate and redirects to it
func createTLSFetcher(timeout time.Duration, server *httptest.Server) *HTTPFetcher {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // For testing only
		},
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &testTransport{
			base:      transport,
			serverURL: server.URL,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	return &HTTPFetcher{
		client:  client,
		timeout: timeout,
	}
}

func TestHTTPFetcher_Fetch_Success(t *testing.T) {
	// Create mock TLS server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/ads.txt", r.URL.Path)
		assert.Equal(t, "AdsTxt-Analyzer/1.0", r.Header.Get("User-Agent"))
		assert.Equal(t, "text/plain", r.Header.Get("Accept"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("google.com, pub-123456, DIRECT, abc123\nexample.com, pub-789, RESELLER"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	// Use a dummy domain - the transport will rewrite it to the test server
	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	require.NoError(t, err)
	assert.Contains(t, content, "google.com, pub-123456, DIRECT, abc123")
	assert.Contains(t, content, "example.com, pub-789, RESELLER")
}

func TestHTTPFetcher_Fetch_NotFound(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
	assert.ErrorIs(t, err, models.ErrDomainNotFound)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestHTTPFetcher_Fetch_EmptyDomain(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)
	ctx := context.Background()

	content, err := fetcher.Fetch(ctx, "")

	assert.Empty(t, content)
	assert.ErrorIs(t, err, models.ErrInvalidDomain)
}

func TestHTTPFetcher_Fetch_Timeout(t *testing.T) {
	// Create server that sleeps longer than timeout
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create fetcher with very short timeout
	fetcher := createTLSFetcher(100*time.Millisecond, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
	// The error should be a timeout-related error
	assert.Contains(t, err.Error(), "failed to fetch ads.txt")
}

func TestHTTPFetcher_Fetch_ContextTimeout(t *testing.T) {
	// Create server that sleeps
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
	assert.ErrorIs(t, err, models.ErrFetchTimeout)
}

func TestHTTPFetcher_Fetch_UnexpectedStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"Internal Server Error", http.StatusInternalServerError},
		{"Bad Gateway", http.StatusBadGateway},
		{"Service Unavailable", http.StatusServiceUnavailable},
		{"Forbidden", http.StatusForbidden},
		{"Unauthorized", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			fetcher := createTLSFetcher(5*time.Second, server)
			ctx := context.Background()

			domain := strings.TrimPrefix(server.URL, "https://")

			content, err := fetcher.Fetch(ctx, domain)

			assert.Empty(t, content)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unexpected HTTP status")
			assert.Contains(t, err.Error(), fmt.Sprintf("%d", tt.statusCode))
		})
	}
}

func TestHTTPFetcher_Fetch_BodySizeLimit(t *testing.T) {
	// Create server that returns content larger than 1MB
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Write 2MB of data
		largeContent := strings.Repeat("a", 2*1024*1024)
		_, _ = w.Write([]byte(largeContent))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestHTTPFetcher_NormalizeDomain(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain domain", "example.com", "example.com"},
		{"with http", "http://example.com", "example.com"},
		{"with https", "https://example.com", "example.com"},
		{"with path", "example.com/path", "example.com"},
		{"with http and path", "http://example.com/path", "example.com"},
		{"with https and path", "https://example.com/path", "example.com"},
		{"with subdomain", "www.example.com", "www.example.com"},
		{"with port (should be removed)", "example.com:8080", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fetcher.normalizeDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTTPFetcher_Fetch_Redirect(t *testing.T) {
	// Create server that redirects once then serves content
	redirectCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if redirectCount == 0 {
			redirectCount++
			http.Redirect(w, r, "/ads.txt", http.StatusMovedPermanently)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("google.com, pub-123456, DIRECT, abc123"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	require.NoError(t, err)
	assert.Contains(t, content, "google.com, pub-123456, DIRECT, abc123")
}

func TestHTTPFetcher_Fetch_TooManyRedirects(t *testing.T) {
	// Create server that redirects to itself
	var redirectServer *httptest.Server
	redirectServer = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectServer.URL+"/ads.txt", http.StatusMovedPermanently)
	}))
	defer redirectServer.Close()

	fetcher := createTLSFetcher(5*time.Second, redirectServer)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch ads.txt")
}

func TestHTTPFetcher_Fetch_EmptyResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Don't write any body
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	require.NoError(t, err)
	assert.Empty(t, content)
}

func TestHTTPFetcher_Fetch_MultipleRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf("Request #%d", requestCount)))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := strings.TrimPrefix(server.URL, "https://")

	// Make multiple requests
	for i := 1; i <= 3; i++ {
		content, err := fetcher.Fetch(ctx, domain)
		require.NoError(t, err)
		assert.Contains(t, content, fmt.Sprintf("Request #%d", i))
	}

	assert.Equal(t, 3, requestCount)
}

func TestHTTPFetcher_Fetch_SpecialCharactersInContent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# Comment line\n google.com, pub-123, DIRECT\n\n"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	require.NoError(t, err)
	assert.Contains(t, content, "# Comment line")
	assert.Contains(t, content, "google.com, pub-123, DIRECT")
}

func TestNewHTTPFetcher_PublicConstructor(t *testing.T) {
	// Test the public constructor
	fetcher := NewHTTPFetcher(10 * time.Second)
	assert.NotNil(t, fetcher)

	// Verify timeout is set correctly by checking the internal implementation
	httpFetcher, ok := fetcher.(*HTTPFetcher)
	require.True(t, ok)
	assert.Equal(t, 10*time.Second, httpFetcher.timeout)
	assert.NotNil(t, httpFetcher.client)
}

func TestHTTPFetcher_ReadBodyWithLimit(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	tests := []struct {
		name      string
		content   string
		maxSize   int64
		expectErr bool
	}{
		{"within limit", "small content", 1000, false},
		{"at limit minus 1", strings.Repeat("a", 999), 1000, false},
		{"exceeds limit", strings.Repeat("a", 1001), 1000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			data, err := fetcher.readBodyWithLimit(reader, tt.maxSize)

			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "too large")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.content, string(data))
			}
		})
	}
}

func TestHTTPFetcher_Fetch_CancelledContext(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("content"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	domain := "example.com"

	content, err := fetcher.Fetch(ctx, domain)

	assert.Empty(t, content)
	assert.Error(t, err)
}

func BenchmarkHTTPFetcher_Fetch(b *testing.B) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("google.com, pub-123456, DIRECT, abc123"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()
	domain := "example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = fetcher.Fetch(ctx, domain)
	}
}

func BenchmarkHTTPFetcher_NormalizeDomain(b *testing.B) {
	fetcher := newHTTPFetcher(5 * time.Second)
	domains := []string{
		"example.com",
		"http://example.com",
		"https://example.com/path",
		"www.example.com:8080",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domain := domains[i%len(domains)]
		_ = fetcher.normalizeDomain(domain)
	}
}

func TestHTTPFetcher_NormalizeDomain_EdgeCases(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"domain with query params", "example.com?param=value", "example.com"},
		{"domain with fragment", "example.com#section", "example.com"},
		{"domain with both path and query", "example.com/path?query=1", "example.com"},
		{"just hostname no path separator", "localhost", "localhost"},
		{"ip address", "192.168.1.1", "192.168.1.1"},
		{"ip with port", "192.168.1.1:8080", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fetcher.normalizeDomain(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTTPFetcher_ReadBodyWithLimit_JustUnderLimit(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	// Test reading just under the limit (should succeed)
	content := strings.Repeat("a", 999)
	reader := strings.NewReader(content)
	data, err := fetcher.readBodyWithLimit(reader, 1000)

	require.NoError(t, err)
	assert.Equal(t, content, string(data))
	assert.Equal(t, 999, len(data))
}

func TestHTTPFetcher_ReadBodyWithLimit_ReadError(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	// Create a reader that will return an error
	errorReader := &errorReader{err: fmt.Errorf("read error")}

	data, err := fetcher.readBodyWithLimit(errorReader, 1000)

	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "read error")
}

// errorReader is a helper that always returns an error when read
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func TestHTTPFetcher_CheckRedirect_ExactlyFiveRedirects(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	// Simulate 4 previous redirects (5th one should be allowed)
	req := &http.Request{}
	via := make([]*http.Request, 4)

	err := fetcher.client.CheckRedirect(req, via)
	assert.NoError(t, err)
}

func TestHTTPFetcher_CheckRedirect_TooManyRedirects(t *testing.T) {
	fetcher := newHTTPFetcher(5 * time.Second)

	// Simulate 5 previous redirects (6th one should be rejected)
	req := &http.Request{}
	via := make([]*http.Request, 5)

	err := fetcher.client.CheckRedirect(req, via)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many redirects")
}

func TestHTTPFetcher_Fetch_WithDifferentProtocols(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("google.com, pub-123, DIRECT"))
	}))
	defer server.Close()

	fetcher := createTLSFetcher(5*time.Second, server)
	ctx := context.Background()

	tests := []struct {
		name   string
		domain string
	}{
		{"plain domain", "example.com"},
		{"with http prefix", "http://example.com"},
		{"with https prefix", "https://example.com"},
		{"with www", "www.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := fetcher.Fetch(ctx, tt.domain)
			require.NoError(t, err)
			assert.Contains(t, content, "google.com")
		})
	}
}
