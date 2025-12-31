package domainAnalysis

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"Perion_Assignment/internal/cache/domainCache"
	"Perion_Assignment/internal/fetcher"
	"Perion_Assignment/internal/logger"
	"Perion_Assignment/internal/models"
	"Perion_Assignment/internal/parser"
)

// Service implements the AnalysisService interface
type Service struct {
	parser        parser.Service
	fetcher       fetcher.Service
	domainCache   domainCache.Service
	logger        logger.Service
	maxConcurrent int
}

// NewService creates a new analysis service
func NewService(
	parser parser.Service,
	fetcher fetcher.Service,
	domainCache domainCache.Service,
	logger logger.Service,
	maxConcurrent int,
) AnalysisService {
	return &Service{
		parser:        parser,
		fetcher:       fetcher,
		domainCache:   domainCache,
		logger:        logger,
		maxConcurrent: maxConcurrent,
	}
}

// AnalyzeDomain analyzes a single domain's ads.txt file
func (s *Service) AnalyzeDomain(ctx context.Context, domain string) (*models.DomainAnalysis, error) {
	start := time.Now()

	// Try to get from domain cache first
	if cached, err := s.domainCache.Get(ctx, domain); err == nil {
		s.logger.LogSuccess(ctx, logger.OpCacheHit, domain, "Retrieved analysis from cache", map[string]interface{}{
			"duration_ms": time.Since(start).Milliseconds(),
		})

		// Mark as cached and return
		cached.Cached = true
		return cached, nil
	}

	s.logger.LogInfo(ctx, logger.OpCacheMiss, fmt.Sprintf("Cache miss for domain: %s", domain), map[string]interface{}{
		"domain": domain,
	})

	// Fetch ads.txt content
	content, err := s.fetcher.Fetch(ctx, domain)
	if err != nil {
		s.logger.LogError(ctx, logger.OpFetchAdsTxt, domain, "Failed to fetch ads.txt", err, models.LogSeverityMedium, map[string]interface{}{
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return nil, models.NewDomainError(domain, "failed to fetch ads.txt", err)
	}

	s.logger.LogSuccess(ctx, logger.OpFetchAdsTxt, domain, "Successfully fetched ads.txt", map[string]interface{}{
		"content_size": len(content),
		"duration_ms":  time.Since(start).Milliseconds(),
	})

	// Parse ads.txt content
	entries, err := s.parser.Parse(content)
	if err != nil {
		s.logger.LogError(ctx, logger.OpParseAdsTxt, domain, "Failed to parse ads.txt", err, models.LogSeverityMedium, map[string]interface{}{
			"content_size": len(content),
			"duration_ms":  time.Since(start).Milliseconds(),
		})
		return nil, models.NewDomainError(domain, "failed to parse ads.txt", err)
	}

	s.logger.LogSuccess(ctx, logger.OpParseAdsTxt, domain, "Successfully parsed ads.txt", map[string]interface{}{
		"entries_count": len(entries),
		"duration_ms":   time.Since(start).Milliseconds(),
	})

	// Build analysis result
	analysis := s.buildAnalysis(domain, entries)

	// Cache the result
	if err := s.domainCache.Set(ctx, domain, analysis, 0); err != nil {
		s.logger.LogError(ctx, "cache_set", domain, "Failed to cache analysis result", err, models.LogSeverityLow, map[string]interface{}{
			"duration_ms": time.Since(start).Milliseconds(),
		})
		// Don't fail the request if caching fails
	}

	s.logger.LogSuccess(ctx, logger.OpDomainAnalysis, domain, "Successfully completed domain analysis", map[string]interface{}{
		"total_advertisers": analysis.TotalAdvertisers,
		"duration_ms":       time.Since(start).Milliseconds(),
		"cached":            false,
	})

	return analysis, nil
}

// AnalyzeDomains analyzes multiple domains concurrently
func (s *Service) AnalyzeDomains(ctx context.Context, domains []string) (*models.BatchAnalysisResponse, error) {
	start := time.Now()

	s.logger.LogInfo(ctx, logger.OpBatchAnalysis, fmt.Sprintf("Starting batch analysis of %d domains", len(domains)), map[string]interface{}{
		"domains_count": len(domains),
		"domains":       domains,
	})

	if len(domains) == 0 {
		return &models.BatchAnalysisResponse{
			Results:          []models.DomainResult{},
			Summary:          models.BatchSummary{Total: 0, Succeeded: 0, Failed: 0},
			Advertisers:      []models.AdvertiserInfo{},
			TotalAdvertisers: 0,
			Timestamp:        time.Now().UTC(),
		}, nil
	}

	// Create results channel and response aggregator
	resultsChan := make(chan models.DomainResult, len(domains))
	responseChan := make(chan *models.BatchAnalysisResponse, 1)

	// Start response aggregator goroutine
	go s.aggregateResults(resultsChan, len(domains), responseChan)

	// Use semaphore to limit concurrent operations
	sem := make(chan struct{}, s.maxConcurrent)
	var wg sync.WaitGroup

	// Process domains concurrently
	for _, domain := range domains {
		wg.Add(1)

		go func(dom string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create context with timeout for individual domain
			domainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			var result models.DomainResult
			analysis, err := s.AnalyzeDomain(domainCtx, dom)
			if err != nil {
				result = models.DomainResult{
					Domain:    dom,
					Error:     err.Error(),
					Success:   false,
					Cached:    false, // Explicitly set cached to false for failed requests
					Timestamp: time.Now().UTC(),
				}

				s.logger.LogError(domainCtx, logger.OpBatchAnalysis, dom, "Failed to analyze domain in batch", err, models.LogSeverityMedium, nil)
			} else {
				result = models.DomainResult{
					Domain:           dom,
					TotalAdvertisers: analysis.TotalAdvertisers,
					Advertisers:      analysis.Advertisers,
					Cached:           analysis.Cached, // This will be true or false based on cache hit/miss
					Success:          true,
					Timestamp:        analysis.Timestamp,
				}
			}

			// Send result to aggregator
			resultsChan <- result
		}(domain)
	}

	// Wait for all workers to complete, then close results channel
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Wait for aggregated response
	response := <-responseChan

	s.logger.LogSuccess(ctx, logger.OpBatchAnalysis, "", "Completed batch analysis", map[string]interface{}{
		"total_domains":       response.Summary.Total,
		"successful":          response.Summary.Succeeded,
		"failed":              response.Summary.Failed,
		"total_advertisers":   response.TotalAdvertisers,
		"success_rate":        float64(response.Summary.Succeeded) / float64(response.Summary.Total),
		"duration_ms":         time.Since(start).Milliseconds(),
	})

	return response, nil
}

// buildAnalysis creates a DomainAnalysis from parsed entries
func (s *Service) buildAnalysis(domain string, entries []models.AdsTxtEntry) *models.DomainAnalysis {
	// Count advertisers
	counts := s.parser.CountAdvertisers(entries)

	// Convert to sorted slice for consistent output and calculate total
	var advertisers []models.AdvertiserInfo
	totalCount := 0
	for domain, count := range counts {
		advertisers = append(advertisers, models.AdvertiserInfo{
			Domain: domain,
			Count:  count,
		})
		totalCount += count
	}

	// Sort by count (descending), then by domain name (ascending)
	sort.Slice(advertisers, func(i, j int) bool {
		if advertisers[i].Count == advertisers[j].Count {
			return advertisers[i].Domain < advertisers[j].Domain
		}
		return advertisers[i].Count > advertisers[j].Count
	})

	return &models.DomainAnalysis{
		Domain:           domain,
		TotalAdvertisers: totalCount, // Sum of all advertiser counts
		Advertisers:      advertisers,
		Cached:           false, // Will be set to true if served from cache
		Timestamp:        time.Now().UTC(),
	}
}


// aggregateResults processes results from workers concurrently and builds the final response
func (s *Service) aggregateResults(resultsChan <-chan models.DomainResult, totalDomains int, responseChan chan<- *models.BatchAnalysisResponse) {
	// Initialize response components
	results := make([]models.DomainResult, 0, totalDomains)
	advertiserCounts := make(map[string]int) // Use map for O(1) aggregation
	summary := models.BatchSummary{Total: totalDomains}

	// Process results as they arrive
	for result := range resultsChan {
		// Add to results array
		results = append(results, result)

		// Update summary statistics
		if result.Success {
			summary.Succeeded++
			
			// Aggregate advertisers efficiently using map
			for _, advertiser := range result.Advertisers {
				advertiserCounts[advertiser.Domain] += advertiser.Count
			}
		} else {
			summary.Failed++
		}
	}

	// Convert map to sorted array and calculate total count
	advertisers := make([]models.AdvertiserInfo, 0, len(advertiserCounts))
	totalCount := 0
	for domain, count := range advertiserCounts {
		advertisers = append(advertisers, models.AdvertiserInfo{
			Domain: domain,
			Count:  count,
		})
		totalCount += count
	}

	// Sort advertisers by count (descending), then by domain (ascending)
	sort.Slice(advertisers, func(i, j int) bool {
		if advertisers[i].Count == advertisers[j].Count {
			return advertisers[i].Domain < advertisers[j].Domain
		}
		return advertisers[i].Count > advertisers[j].Count
	})

	// Send completed response
	responseChan <- &models.BatchAnalysisResponse{
		Results:          results,
		Summary:          summary,
		Advertisers:      advertisers,
		TotalAdvertisers: totalCount, // Sum of all advertiser counts across all domains
		Timestamp:        time.Now().UTC(),
	}
}