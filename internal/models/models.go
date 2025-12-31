package models

import (
	"time"
)

// AdvertiserInfo represents a single advertiser's information
type AdvertiserInfo struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// DomainAnalysis represents the complete analysis of a domain's ads.txt
type DomainAnalysis struct {
	Domain           string            `json:"domain"`
	TotalAdvertisers int               `json:"total_advertisers"`
	Advertisers      []AdvertiserInfo  `json:"advertisers"`
	Cached           bool              `json:"cached"`
	Timestamp        time.Time         `json:"timestamp"`
}

// BatchAnalysisRequest represents a request for analyzing multiple domains
type BatchAnalysisRequest struct {
	Domains []string `json:"domains"`
}

// DomainResult represents a single domain result in batch analysis
type DomainResult struct {
	Domain           string             `json:"domain"`
	TotalAdvertisers int                `json:"total_advertisers,omitempty"`
	Advertisers      []AdvertiserInfo   `json:"advertisers,omitempty"`
	Cached           bool               `json:"cached"`
	Error            string             `json:"error,omitempty"`
	Success          bool               `json:"success"`
	Timestamp        time.Time          `json:"timestamp,omitempty"`
}

// BatchSummary provides summary statistics for batch operations
type BatchSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// BatchAnalysisResponse represents the response for batch analysis
type BatchAnalysisResponse struct {
	Results          []DomainResult   `json:"results"`
	Summary          BatchSummary     `json:"summary"`
	Advertisers      []AdvertiserInfo `json:"advertisers"` // Summarized across all domains
	TotalAdvertisers int              `json:"total_advertisers"`
	Timestamp        time.Time        `json:"timestamp"`
}

// AdsTxtEntry represents a single line in an ads.txt file
type AdsTxtEntry struct {
	ExchangeDomain    string
	PublisherID       string
	AccountType       string
	CertificationAuth string
}

// LogSeverity represents the severity level of a log entry
type LogSeverity string

const (
	LogSeverityLow    LogSeverity = "low"
	LogSeverityMedium LogSeverity = "medium"
	LogSeverityHigh   LogSeverity = "high"
)

// ProcessType represents the type of process that created the log
type ProcessType string

const (
	ProcessTypeRequest  ProcessType = "request"
	ProcessTypeInternal ProcessType = "internal"
)

// LogEvent represents a process-specific logging context
type LogEvent struct {
	ProcessID   string      `json:"process_id"`
	ProcessType ProcessType `json:"process_type"`
	StartTime   time.Time   `json:"start_time"`
	ClientIP    string      `json:"client_ip,omitempty"`
}

// LogEntry represents a structured log entry for database storage
type LogEntry struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Severity    LogSeverity            `json:"severity,omitempty"`
	Message     string                 `json:"message"`
	Operation   string                 `json:"operation"`
	TargetName  string                 `json:"target_name,omitempty"` // Renamed from Domain
	ProcessID   string                 `json:"process_id"`
	ProcessType ProcessType            `json:"process_type"`
	ClientIP    string                 `json:"client_ip,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}


