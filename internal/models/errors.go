package models

import (
	"errors"
	"fmt"
)

var (
	// ErrDomainNotFound indicates that the domain's ads.txt file could not be found
	ErrDomainNotFound = errors.New("ads.txt not found for domain")
	
	// ErrInvalidDomain indicates that the provided domain is invalid
	ErrInvalidDomain = errors.New("invalid domain format")
	
	// ErrFetchTimeout indicates that fetching ads.txt timed out
	ErrFetchTimeout = errors.New("timeout while fetching ads.txt")
	
	// ErrRateLimitExceeded indicates that rate limit has been exceeded
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	
	// ErrCacheUnavailable indicates that cache service is unavailable
	ErrCacheUnavailable = errors.New("cache service unavailable")
	
	// ErrInvalidAdsTxtFormat indicates that ads.txt content is malformed
	ErrInvalidAdsTxtFormat = errors.New("invalid ads.txt format")
)

// DomainError represents an error specific to a domain operation
type DomainError struct {
	Domain  string
	Message string
	Err     error
}

func (e *DomainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("domain %s: %s: %v", e.Domain, e.Message, e.Err)
	}
	return fmt.Sprintf("domain %s: %s", e.Domain, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// NewDomainError creates a new domain-specific error
func NewDomainError(domain, message string, err error) *DomainError {
	return &DomainError{
		Domain:  domain,
		Message: message,
		Err:     err,
	}
}