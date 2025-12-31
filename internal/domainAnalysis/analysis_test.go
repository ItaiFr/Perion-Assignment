package domainAnalysis

import (
	mocks2 "Perion_Assignment/internal/mocks"
	"context"
	"errors"
	"testing"
	"time"

	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_AnalyzeDomain_CacheHit(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()

	expectedAnalysis := &models.DomainAnalysis{
		Domain:           domain,
		TotalAdvertisers: 5,
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 3},
			{Domain: "facebook.com", Count: 2},
		},
		Cached:    false, // Will be set to true by the service
		Timestamp: time.Now().UTC(),
	}

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(expectedAnalysis, nil)
	mockLogger.On("LogSuccess", ctx, "cache_hit", domain, "Retrieved analysis from cache", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain, result.Domain)
	assert.Equal(t, 5, result.TotalAdvertisers)
	assert.True(t, result.Cached) // Should be set to true by service
	assert.Equal(t, 2, len(result.Advertisers))

	// Verify all mocks were called as expected
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)

	// Verify fetcher and parser were NOT called (cache hit)
	mockFetcher.AssertNotCalled(t, "Fetch")
	mockParser.AssertNotCalled(t, "Parse")
}

func TestService_AnalyzeDomain_CacheMissSuccess(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()
	adsTxtContent := "google.com, pub-123, DIRECT, f08c47fec0942fa0\nfacebook.com, pub-456, RESELLER"

	entries := []models.AdsTxtEntry{
		{ExchangeDomain: "google.com", PublisherID: "pub-123", AccountType: "DIRECT", CertificationAuth: "f08c47fec0942fa0"},
		{ExchangeDomain: "facebook.com", PublisherID: "pub-456", AccountType: "RESELLER"},
	}

	advertiserCounts := map[string]int{
		"google.com":   1,
		"facebook.com": 1,
	}

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", ctx, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()

	mockFetcher.On("Fetch", ctx, domain).Return(adsTxtContent, nil)
	mockLogger.On("LogSuccess", ctx, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()

	mockParser.On("Parse", adsTxtContent).Return(entries, nil)
	mockLogger.On("LogSuccess", ctx, "parse_ads_txt", domain, "Successfully parsed ads.txt", mock.Anything).Return()

	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)

	mockCache.On("Set", ctx, domain, mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", ctx, "domain_analysis", domain, "Successfully completed domain analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain, result.Domain)
	assert.Equal(t, 2, result.TotalAdvertisers) // Sum of counts: 1 + 1
	assert.False(t, result.Cached)              // Not from cache
	assert.Equal(t, 2, len(result.Advertisers))

	// Verify advertisers are sorted by count desc, then domain asc
	assert.Equal(t, "facebook.com", result.Advertisers[0].Domain) // alphabetically first when counts are equal
	assert.Equal(t, "google.com", result.Advertisers[1].Domain)

	// Verify all mocks were called
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomain_FetchError(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()
	fetchError := errors.New("network timeout")

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", ctx, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()

	mockFetcher.On("Fetch", ctx, domain).Return("", fetchError)
	mockLogger.On("LogError", ctx, "fetch_ads_txt", domain, "Failed to fetch ads.txt", fetchError, models.LogSeverityMedium, mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	// Check error is wrapped as DomainError
	var domainError *models.DomainError
	assert.ErrorAs(t, err, &domainError)
	assert.Equal(t, domain, domainError.Domain)
	assert.Contains(t, domainError.Message, "failed to fetch ads.txt")

	// Verify mocks
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)

	// Parser should not be called
	mockParser.AssertNotCalled(t, "Parse")
}

func TestService_AnalyzeDomain_ParseError(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()
	adsTxtContent := "invalid content"
	parseError := errors.New("invalid format")

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", ctx, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()

	mockFetcher.On("Fetch", ctx, domain).Return(adsTxtContent, nil)
	mockLogger.On("LogSuccess", ctx, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()

	mockParser.On("Parse", adsTxtContent).Return(nil, parseError)
	mockLogger.On("LogError", ctx, "parse_ads_txt", domain, "Failed to parse ads.txt", parseError, models.LogSeverityMedium, mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	// Check error is wrapped as DomainError
	var domainError *models.DomainError
	assert.ErrorAs(t, err, &domainError)
	assert.Equal(t, domain, domainError.Domain)
	assert.Contains(t, domainError.Message, "failed to parse ads.txt")

	// Verify mocks
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomain_CacheSetError(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()
	adsTxtContent := "google.com, pub-123, DIRECT"
	cacheError := errors.New("cache unavailable")

	entries := []models.AdsTxtEntry{
		{ExchangeDomain: "google.com", PublisherID: "pub-123", AccountType: "DIRECT"},
	}

	advertiserCounts := map[string]int{
		"google.com": 1,
	}

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", ctx, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()

	mockFetcher.On("Fetch", ctx, domain).Return(adsTxtContent, nil)
	mockLogger.On("LogSuccess", ctx, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()

	mockParser.On("Parse", adsTxtContent).Return(entries, nil)
	mockLogger.On("LogSuccess", ctx, "parse_ads_txt", domain, "Successfully parsed ads.txt", mock.Anything).Return()

	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)

	// Cache set fails but doesn't break the flow
	mockCache.On("Set", ctx, domain, mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(cacheError)
	mockLogger.On("LogError", ctx, "cache_set", domain, "Failed to cache analysis result", cacheError, models.LogSeverityLow, mock.Anything).Return()
	mockLogger.On("LogSuccess", ctx, "domain_analysis", domain, "Successfully completed domain analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert - should succeed despite cache error
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain, result.Domain)
	assert.Equal(t, 1, result.TotalAdvertisers)
	assert.False(t, result.Cached)

	// Verify all mocks were called
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomain_ComplexAdvertiserSorting(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	domain := "example.com"
	ctx := context.Background()
	adsTxtContent := "test content"

	entries := []models.AdsTxtEntry{}

	// Create complex advertiser counts for sorting test
	advertiserCounts := map[string]int{
		"google.com":   5, // highest count
		"amazon.com":   3, // second highest
		"facebook.com": 3, // same as amazon, should be sorted alphabetically
		"apple.com":    1, // lowest count
		"yahoo.com":    1, // same as apple, should be sorted alphabetically
	}

	// Setup mocks
	mockCache.On("Get", ctx, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", ctx, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()

	mockFetcher.On("Fetch", ctx, domain).Return(adsTxtContent, nil)
	mockLogger.On("LogSuccess", ctx, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()

	mockParser.On("Parse", adsTxtContent).Return(entries, nil)
	mockLogger.On("LogSuccess", ctx, "parse_ads_txt", domain, "Successfully parsed ads.txt", mock.Anything).Return()

	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)

	mockCache.On("Set", ctx, domain, mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", ctx, "domain_analysis", domain, "Successfully completed domain analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomain(ctx, domain)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, domain, result.Domain)
	assert.Equal(t, 13, result.TotalAdvertisers) // 5 + 3 + 3 + 1 + 1

	// Verify sorting: count desc, then domain asc
	expectedOrder := []struct {
		domain string
		count  int
	}{
		{"google.com", 5},   // highest count
		{"amazon.com", 3},   // same count as facebook, but alphabetically first
		{"facebook.com", 3}, // same count as amazon, but alphabetically second
		{"apple.com", 1},    // same count as yahoo, but alphabetically first
		{"yahoo.com", 1},    // same count as apple, but alphabetically second
	}

	require.Equal(t, len(expectedOrder), len(result.Advertisers))
	for i, expected := range expectedOrder {
		assert.Equal(t, expected.domain, result.Advertisers[i].Domain, "Incorrect order at position %d", i)
		assert.Equal(t, expected.count, result.Advertisers[i].Count, "Incorrect count at position %d", i)
	}

	// Verify all mocks
	mockCache.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_buildAnalysis(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	service := &Service{parser: mockParser}

	domain := "example.com"
	entries := []models.AdsTxtEntry{
		{ExchangeDomain: "google.com"},
		{ExchangeDomain: "facebook.com"},
	}

	advertiserCounts := map[string]int{
		"google.com":   3,
		"facebook.com": 2,
	}

	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)

	// Act
	result := service.buildAnalysis(domain, entries)

	// Assert
	require.NotNil(t, result)
	assert.Equal(t, domain, result.Domain)
	assert.Equal(t, 5, result.TotalAdvertisers) // 3 + 2
	assert.False(t, result.Cached)
	assert.Equal(t, 2, len(result.Advertisers))

	// Verify sorting by count (descending)
	assert.Equal(t, "google.com", result.Advertisers[0].Domain)
	assert.Equal(t, 3, result.Advertisers[0].Count)
	assert.Equal(t, "facebook.com", result.Advertisers[1].Domain)
	assert.Equal(t, 2, result.Advertisers[1].Count)

	// Verify timestamp is recent
	assert.WithinDuration(t, time.Now().UTC(), result.Timestamp, 5*time.Second)

	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomains_EmptySlice(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	ctx := context.Background()
	domains := []string{}

	mockLogger.On("LogInfo", ctx, "batch_analysis", "Starting batch analysis of 0 domains", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Summary.Total)
	assert.Equal(t, 0, result.Summary.Succeeded)
	assert.Equal(t, 0, result.Summary.Failed)
	assert.Equal(t, 0, result.TotalAdvertisers)
	assert.Empty(t, result.Results)
	assert.Empty(t, result.Advertisers)

	mockLogger.AssertExpectations(t)
}

func TestService_AnalyzeDomains_SingleDomainSuccess(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	ctx := context.Background()
	domains := []string{"example.com"}
	domain := domains[0]

	entries := []models.AdsTxtEntry{
		{ExchangeDomain: "google.com"},
		{ExchangeDomain: "facebook.com"},
	}

	advertiserCounts := map[string]int{
		"google.com":   2,
		"facebook.com": 1,
	}

	// Setup mocks for AnalyzeDomain call
	mockLogger.On("LogInfo", ctx, "batch_analysis", mock.AnythingOfType("string"), mock.Anything).Return()
	mockCache.On("Get", mock.Anything, domain).Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, domain).Return("test content", nil)
	mockLogger.On("LogSuccess", mock.Anything, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()
	mockParser.On("Parse", "test content").Return(entries, nil)
	mockLogger.On("LogSuccess", mock.Anything, "parse_ads_txt", domain, "Successfully parsed ads.txt", mock.Anything).Return()
	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)
	mockCache.On("Set", mock.Anything, domain, mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", domain, "Successfully completed domain analysis", mock.Anything).Return()
	mockLogger.On("LogSuccess", ctx, "batch_analysis", "", "Completed batch analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check summary
	assert.Equal(t, 1, result.Summary.Total)
	assert.Equal(t, 1, result.Summary.Succeeded)
	assert.Equal(t, 0, result.Summary.Failed)

	// Check total advertisers (sum across all domains)
	assert.Equal(t, 3, result.TotalAdvertisers) // 2 + 1

	// Check results
	require.Len(t, result.Results, 1)
	domainResult := result.Results[0]
	assert.Equal(t, domain, domainResult.Domain)
	assert.True(t, domainResult.Success)
	assert.Equal(t, 3, domainResult.TotalAdvertisers)
	assert.False(t, domainResult.Cached)
	assert.Empty(t, domainResult.Error)

	// Check aggregated advertisers
	require.Len(t, result.Advertisers, 2)
	assert.Equal(t, "google.com", result.Advertisers[0].Domain)
	assert.Equal(t, 2, result.Advertisers[0].Count)
	assert.Equal(t, "facebook.com", result.Advertisers[1].Domain)
	assert.Equal(t, 1, result.Advertisers[1].Count)

	// Verify all mocks
	mockLogger.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomains_MultipleDomainsMixedResults(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	ctx := context.Background()
	domains := []string{"example.com", "test.com", "fail.com"}

	// Setup mocks for batch analysis start
	mockLogger.On("LogInfo", ctx, "batch_analysis", "Starting batch analysis of 3 domains", mock.Anything).Return()

	// example.com - success from cache
	cachedAnalysis := &models.DomainAnalysis{
		Domain:           "example.com",
		TotalAdvertisers: 2,
		Advertisers: []models.AdvertiserInfo{
			{Domain: "google.com", Count: 2},
		},
		Cached: false,
	}
	mockCache.On("Get", mock.Anything, "example.com").Return(cachedAnalysis, nil)
	mockLogger.On("LogSuccess", mock.Anything, "cache_hit", "example.com", "Retrieved analysis from cache", mock.Anything).Return()

	// test.com - success from fresh fetch
	entries := []models.AdsTxtEntry{{ExchangeDomain: "facebook.com"}}
	advertiserCounts := map[string]int{"facebook.com": 1}

	mockCache.On("Get", mock.Anything, "test.com").Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, "test.com").Return("test content", nil)
	mockLogger.On("LogSuccess", mock.Anything, "fetch_ads_txt", "test.com", "Successfully fetched ads.txt", mock.Anything).Return()
	mockParser.On("Parse", "test content").Return(entries, nil)
	mockLogger.On("LogSuccess", mock.Anything, "parse_ads_txt", "test.com", "Successfully parsed ads.txt", mock.Anything).Return()
	mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)
	mockCache.On("Set", mock.Anything, "test.com", mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", "test.com", "Successfully completed domain analysis", mock.Anything).Return()

	// fail.com - fetch error
	fetchError := errors.New("network timeout")
	mockCache.On("Get", mock.Anything, "fail.com").Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, "fail.com").Return("", fetchError)
	mockLogger.On("LogError", mock.Anything, "fetch_ads_txt", "fail.com", "Failed to fetch ads.txt", fetchError, models.LogSeverityMedium, mock.Anything).Return()
	mockLogger.On("LogError", mock.Anything, "batch_analysis", "fail.com", "Failed to analyze domain in batch", mock.AnythingOfType("*models.DomainError"), models.LogSeverityMedium, mock.Anything).Return()

	// Final batch completion log
	mockLogger.On("LogSuccess", ctx, "batch_analysis", "", "Completed batch analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check summary
	assert.Equal(t, 3, result.Summary.Total)
	assert.Equal(t, 2, result.Summary.Succeeded)
	assert.Equal(t, 1, result.Summary.Failed)

	// Check aggregated advertisers (only from successful domains)
	assert.Equal(t, 3, result.TotalAdvertisers) // google.com: 2 + facebook.com: 1
	require.Len(t, result.Advertisers, 2)

	// Verify sorting of aggregated advertisers
	assert.Equal(t, "google.com", result.Advertisers[0].Domain)
	assert.Equal(t, 2, result.Advertisers[0].Count)
	assert.Equal(t, "facebook.com", result.Advertisers[1].Domain)
	assert.Equal(t, 1, result.Advertisers[1].Count)

	// Check individual results
	require.Len(t, result.Results, 3)

	// Find results by domain (order is not guaranteed due to concurrency)
	resultsByDomain := make(map[string]models.DomainResult)
	for _, r := range result.Results {
		resultsByDomain[r.Domain] = r
	}

	// Check example.com (cached)
	exampleResult := resultsByDomain["example.com"]
	assert.True(t, exampleResult.Success)
	assert.True(t, exampleResult.Cached)
	assert.Equal(t, 2, exampleResult.TotalAdvertisers)
	assert.Empty(t, exampleResult.Error)

	// Check test.com (fresh)
	testResult := resultsByDomain["test.com"]
	assert.True(t, testResult.Success)
	assert.False(t, testResult.Cached)
	assert.Equal(t, 1, testResult.TotalAdvertisers)
	assert.Empty(t, testResult.Error)

	// Check fail.com (failed)
	failResult := resultsByDomain["fail.com"]
	assert.False(t, failResult.Success)
	assert.False(t, failResult.Cached)
	assert.Equal(t, 0, failResult.TotalAdvertisers)
	assert.Contains(t, failResult.Error, "failed to fetch ads.txt")

	// Verify all mocks
	mockLogger.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomains_AllFailed(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	ctx := context.Background()
	domains := []string{"fail1.com", "fail2.com"}

	fetchError := errors.New("network error")

	// Setup mocks for batch analysis start
	mockLogger.On("LogInfo", ctx, "batch_analysis", "Starting batch analysis of 2 domains", mock.Anything).Return()

	// Both domains fail
	for _, domain := range domains {
		mockCache.On("Get", mock.Anything, domain).Return(nil, errors.New("cache miss"))
		mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
		mockFetcher.On("Fetch", mock.Anything, domain).Return("", fetchError)
		mockLogger.On("LogError", mock.Anything, "fetch_ads_txt", domain, "Failed to fetch ads.txt", fetchError, models.LogSeverityMedium, mock.Anything).Return()
		mockLogger.On("LogError", mock.Anything, "batch_analysis", domain, "Failed to analyze domain in batch", mock.AnythingOfType("*models.DomainError"), models.LogSeverityMedium, mock.Anything).Return()
	}

	mockLogger.On("LogSuccess", ctx, "batch_analysis", "", "Completed batch analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check summary - all failed
	assert.Equal(t, 2, result.Summary.Total)
	assert.Equal(t, 0, result.Summary.Succeeded)
	assert.Equal(t, 2, result.Summary.Failed)

	// No advertisers when all failed
	assert.Equal(t, 0, result.TotalAdvertisers)
	assert.Empty(t, result.Advertisers)

	// All results should be failures
	require.Len(t, result.Results, 2)
	for _, domainResult := range result.Results {
		assert.False(t, domainResult.Success)
		assert.False(t, domainResult.Cached)
		assert.Equal(t, 0, domainResult.TotalAdvertisers)
		assert.NotEmpty(t, domainResult.Error)
	}

	// Verify all mocks
	mockLogger.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
}

func TestService_AnalyzeDomains_ConcurrencyLimit(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	// Set max concurrent to 2 to test concurrency limiting
	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 2).(*Service)

	ctx := context.Background()
	domains := []string{"domain1.com", "domain2.com", "domain3.com", "domain4.com"}

	entries := []models.AdsTxtEntry{{ExchangeDomain: "google.com"}}
	advertiserCounts := map[string]int{"google.com": 1}

	// Setup mocks for batch analysis start
	mockLogger.On("LogInfo", ctx, "batch_analysis", "Starting batch analysis of 4 domains", mock.Anything).Return()

	// Setup mocks for each domain (all succeed)
	for _, domain := range domains {
		mockCache.On("Get", mock.Anything, domain).Return(nil, errors.New("cache miss"))
		mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
		mockFetcher.On("Fetch", mock.Anything, domain).Return("test content", nil)
		mockLogger.On("LogSuccess", mock.Anything, "fetch_ads_txt", domain, "Successfully fetched ads.txt", mock.Anything).Return()
		mockParser.On("Parse", "test content").Return(entries, nil)
		mockLogger.On("LogSuccess", mock.Anything, "parse_ads_txt", domain, "Successfully parsed ads.txt", mock.Anything).Return()
		mockParser.On("CountAdvertisers", entries).Return(advertiserCounts)
		mockCache.On("Set", mock.Anything, domain, mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
		mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", domain, "Successfully completed domain analysis", mock.Anything).Return()
	}

	mockLogger.On("LogSuccess", ctx, "batch_analysis", "", "Completed batch analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check summary - all succeeded
	assert.Equal(t, 4, result.Summary.Total)
	assert.Equal(t, 4, result.Summary.Succeeded)
	assert.Equal(t, 0, result.Summary.Failed)

	// Check aggregated results
	assert.Equal(t, 4, result.TotalAdvertisers) // 1 per domain
	require.Len(t, result.Advertisers, 1)       // Only one unique advertiser
	assert.Equal(t, "google.com", result.Advertisers[0].Domain)
	assert.Equal(t, 4, result.Advertisers[0].Count) // Aggregated count

	// All results should be successful
	require.Len(t, result.Results, 4)
	for _, domainResult := range result.Results {
		assert.True(t, domainResult.Success)
		assert.False(t, domainResult.Cached)
		assert.Equal(t, 1, domainResult.TotalAdvertisers)
		assert.Empty(t, domainResult.Error)
	}

	// Verify all mocks were called the expected number of times
	mockLogger.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}

func TestService_AnalyzeDomains_AllSuccess(t *testing.T) {
	// Arrange
	mockParser := &mocks2.MockParser{}
	mockFetcher := &mocks2.MockFetcher{}
	mockCache := &mocks2.MockDomainCache{}
	mockLogger := &mocks2.MockLogger{}

	service := NewService(mockParser, mockFetcher, mockCache, mockLogger, 10).(*Service)

	ctx := context.Background()
	domains := []string{"google.com", "facebook.com", "amazon.com"}

	// Setup different advertiser patterns for each domain
	googleEntries := []models.AdsTxtEntry{
		{ExchangeDomain: "doubleclick.net"},
		{ExchangeDomain: "adsystem.google.com"},
	}
	amazonEntries := []models.AdsTxtEntry{
		{ExchangeDomain: "amazon.com"},
		{ExchangeDomain: "amazon-adsystem.com"},
		{ExchangeDomain: "amazon.com"}, // Duplicate to test aggregation
	}

	googleCounts := map[string]int{"doubleclick.net": 1, "adsystem.google.com": 1}
	amazonCounts := map[string]int{"amazon.com": 2, "amazon-adsystem.com": 1}

	// Setup mocks for batch analysis start
	mockLogger.On("LogInfo", ctx, "batch_analysis", "Starting batch analysis of 3 domains", mock.Anything).Return()

	// google.com - cache miss, fresh fetch
	mockCache.On("Get", mock.Anything, "google.com").Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, "google.com").Return("google content", nil)
	mockLogger.On("LogSuccess", mock.Anything, "fetch_ads_txt", "google.com", "Successfully fetched ads.txt", mock.Anything).Return()
	mockParser.On("Parse", "google content").Return(googleEntries, nil)
	mockLogger.On("LogSuccess", mock.Anything, "parse_ads_txt", "google.com", "Successfully parsed ads.txt", mock.Anything).Return()
	mockParser.On("CountAdvertisers", googleEntries).Return(googleCounts)
	mockCache.On("Set", mock.Anything, "google.com", mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", "google.com", "Successfully completed domain analysis", mock.Anything).Return()

	// facebook.com - cache hit
	cachedFacebookAnalysis := &models.DomainAnalysis{
		Domain:           "facebook.com",
		TotalAdvertisers: 2,
		Advertisers: []models.AdvertiserInfo{
			{Domain: "facebook.com", Count: 2},
		},
		Cached: false,
	}
	mockCache.On("Get", mock.Anything, "facebook.com").Return(cachedFacebookAnalysis, nil)
	mockLogger.On("LogSuccess", mock.Anything, "cache_hit", "facebook.com", "Retrieved analysis from cache", mock.Anything).Return()

	// amazon.com - cache miss, fresh fetch
	mockCache.On("Get", mock.Anything, "amazon.com").Return(nil, errors.New("cache miss"))
	mockLogger.On("LogInfo", mock.Anything, "cache_miss", mock.AnythingOfType("string"), mock.Anything).Return()
	mockFetcher.On("Fetch", mock.Anything, "amazon.com").Return("amazon content", nil)
	mockLogger.On("LogSuccess", mock.Anything, "fetch_ads_txt", "amazon.com", "Successfully fetched ads.txt", mock.Anything).Return()
	mockParser.On("Parse", "amazon content").Return(amazonEntries, nil)
	mockLogger.On("LogSuccess", mock.Anything, "parse_ads_txt", "amazon.com", "Successfully parsed ads.txt", mock.Anything).Return()
	mockParser.On("CountAdvertisers", amazonEntries).Return(amazonCounts)
	mockCache.On("Set", mock.Anything, "amazon.com", mock.AnythingOfType("*models.DomainAnalysis"), time.Duration(0)).Return(nil)
	mockLogger.On("LogSuccess", mock.Anything, "domain_analysis", "amazon.com", "Successfully completed domain analysis", mock.Anything).Return()

	// Final batch completion log
	mockLogger.On("LogSuccess", ctx, "batch_analysis", "", "Completed batch analysis", mock.Anything).Return()

	// Act
	result, err := service.AnalyzeDomains(ctx, domains)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check summary - all succeeded
	assert.Equal(t, 3, result.Summary.Total)
	assert.Equal(t, 3, result.Summary.Succeeded)
	assert.Equal(t, 0, result.Summary.Failed)

	// Check aggregated advertisers (sum across all successful domains)
	// google: 2 + facebook: 2 + amazon: 3 = 7 total
	assert.Equal(t, 7, result.TotalAdvertisers)
	require.Len(t, result.Advertisers, 5) // 5 unique advertiser domains

	// Verify aggregated advertisers are properly sorted
	advertiserMap := make(map[string]int)
	for _, adv := range result.Advertisers {
		advertiserMap[adv.Domain] = adv.Count
	}

	// Check expected counts (aggregated across all domains)
	assert.Equal(t, 1, advertiserMap["doubleclick.net"])
	assert.Equal(t, 1, advertiserMap["adsystem.google.com"])
	assert.Equal(t, 2, advertiserMap["facebook.com"])
	assert.Equal(t, 2, advertiserMap["amazon.com"])
	assert.Equal(t, 1, advertiserMap["amazon-adsystem.com"])

	// Check individual results
	require.Len(t, result.Results, 3)

	// Find results by domain (order is not guaranteed due to concurrency)
	resultsByDomain := make(map[string]models.DomainResult)
	for _, r := range result.Results {
		resultsByDomain[r.Domain] = r
	}

	// Check google.com (fresh fetch)
	googleResult := resultsByDomain["google.com"]
	assert.True(t, googleResult.Success)
	assert.False(t, googleResult.Cached)
	assert.Equal(t, 2, googleResult.TotalAdvertisers) // doubleclick: 1 + adsystem: 1
	assert.Empty(t, googleResult.Error)

	// Check facebook.com (cached)
	facebookResult := resultsByDomain["facebook.com"]
	assert.True(t, facebookResult.Success)
	assert.True(t, facebookResult.Cached)
	assert.Equal(t, 2, facebookResult.TotalAdvertisers)
	assert.Empty(t, facebookResult.Error)

	// Check amazon.com (fresh fetch)
	amazonResult := resultsByDomain["amazon.com"]
	assert.True(t, amazonResult.Success)
	assert.False(t, amazonResult.Cached)
	assert.Equal(t, 3, amazonResult.TotalAdvertisers) // amazon: 2 + amazon-adsystem: 1
	assert.Empty(t, amazonResult.Error)

	// Verify all mocks
	mockLogger.AssertExpectations(t)
	mockCache.AssertExpectations(t)
	mockFetcher.AssertExpectations(t)
	mockParser.AssertExpectations(t)
}
