package parser

import "Perion_Assignment/internal/models"

// Service defines the interface for parsing ads.txt content
// External packages should use this interface, not the concrete implementations
type Service interface {
	Parse(content string) ([]models.AdsTxtEntry, error)
	CountAdvertisers(entries []models.AdsTxtEntry) map[string]int
}