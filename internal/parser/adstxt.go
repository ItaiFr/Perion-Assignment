package parser

import (
	"fmt"
	"regexp"
	"strings"

	"Perion_Assignment/internal/models"
)

// Parser implements the Service interface
type Parser struct {
	// validLineRegex matches valid ads.txt lines
	validLineRegex *regexp.Regexp
}

// NewParser creates a new ads.txt parser
func NewParser() Service {
	return newParser()
}

// newParser creates the concrete implementation
func newParser() *Parser {
	// ads.txt format: exchange_domain, publisher_id, account_type[, certification_authority_id]
	// Lines starting with # are comments
	// Updated to handle case-insensitive account types
	validLineRegex := regexp.MustCompile(`^([^,\s]+),\s*([^,\s]+),\s*(DIRECT|RESELLER|direct|reseller)(?:,\s*([^,\s]+))?\s*$`)
	
	return &Parser{
		validLineRegex: validLineRegex,
	}
}

// Parse parses ads.txt content and returns structured entries
func (p *Parser) Parse(content string) ([]models.AdsTxtEntry, error) {
	if content == "" {
		return nil, fmt.Errorf("%w: empty content", models.ErrInvalidAdsTxtFormat)
	}

	var entries []models.AdsTxtEntry
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		entry, err := p.parseLine(line)
		if err != nil {
			// Log malformed lines but don't fail the entire parsing
			// In production, you might want to log this
			continue
		}
		
		if entry != nil {
			entries = append(entries, *entry)
		}
	}
	
	if len(entries) == 0 {
		return nil, fmt.Errorf("%w: no valid entries found", models.ErrInvalidAdsTxtFormat)
	}
	
	return entries, nil
}

// parseLine parses a single line of ads.txt content
func (p *Parser) parseLine(line string) (*models.AdsTxtEntry, error) {
	matches := p.validLineRegex.FindStringSubmatch(line)
	if len(matches) < 4 {
		return nil, fmt.Errorf("%w: invalid line format: %s", models.ErrInvalidAdsTxtFormat, line)
	}
	
	entry := &models.AdsTxtEntry{
		ExchangeDomain: strings.ToLower(strings.TrimSpace(matches[1])),
		PublisherID:    strings.TrimSpace(matches[2]),
		AccountType:    strings.ToUpper(strings.TrimSpace(matches[3])),
	}
	
	// Certification authority is optional
	if len(matches) > 4 && matches[4] != "" {
		entry.CertificationAuth = strings.TrimSpace(matches[4])
	}
	
	// Validate exchange domain format
	if !p.isValidDomain(entry.ExchangeDomain) {
		return nil, fmt.Errorf("%w: invalid exchange domain: %s", models.ErrInvalidAdsTxtFormat, entry.ExchangeDomain)
	}
	
	return entry, nil
}

// CountAdvertisers counts the occurrences of each advertiser domain
func (p *Parser) CountAdvertisers(entries []models.AdsTxtEntry) map[string]int {
	counts := make(map[string]int)
	
	for _, entry := range entries {
		counts[entry.ExchangeDomain]++
	}
	
	return counts
}

// isValidDomain performs basic domain validation
func (p *Parser) isValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	
	// Handle trailing dot (valid in DNS)
	domain = strings.TrimSuffix(domain, ".")
	
	// Must contain at least one dot and be more than single character
	if !strings.Contains(domain, ".") || len(domain) <= 1 {
		return false
	}
	
	// Basic domain validation with proper character set
	domainRegex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)
	return domainRegex.MatchString(domain)
}