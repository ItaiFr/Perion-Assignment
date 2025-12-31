package mocks

import (
	"Perion_Assignment/internal/models"

	"github.com/stretchr/testify/mock"
)

// MockParser is a mock implementation of parser.Service
type MockParser struct {
	mock.Mock
}

// Parse mocks the Parse method of parser.Service
func (m *MockParser) Parse(content string) ([]models.AdsTxtEntry, error) {
	args := m.Called(content)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.AdsTxtEntry), args.Error(1)
}

// CountAdvertisers mocks the CountAdvertisers method of parser.Service
func (m *MockParser) CountAdvertisers(entries []models.AdsTxtEntry) map[string]int {
	args := m.Called(entries)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(map[string]int)
}