package parser

import (
	"errors"
	"reflect"
	"testing"

	"Perion_Assignment/internal/models"
)

func TestParser_Parse(t *testing.T) {
	parser := newParser()

	tests := []struct {
		name     string
		content  string
		want     []models.AdsTxtEntry
		wantErr  bool
		errType  error
	}{
		{
			name: "valid ads.txt content",
			content: `google.com, pub-123456789, DIRECT, f08c47fec0942fa0
facebook.com, 123456789, RESELLER
appnexus.com, 987654, DIRECT`,
			want: []models.AdsTxtEntry{
				{ExchangeDomain: "google.com", PublisherID: "pub-123456789", AccountType: "DIRECT", CertificationAuth: "f08c47fec0942fa0"},
				{ExchangeDomain: "facebook.com", PublisherID: "123456789", AccountType: "RESELLER"},
				{ExchangeDomain: "appnexus.com", PublisherID: "987654", AccountType: "DIRECT"},
			},
			wantErr: false,
		},
		{
			name: "content with comments and empty lines",
			content: `# This is a comment
google.com, pub-123456789, DIRECT, f08c47fec0942fa0

# Another comment
facebook.com, 123456789, RESELLER

`,
			want: []models.AdsTxtEntry{
				{ExchangeDomain: "google.com", PublisherID: "pub-123456789", AccountType: "DIRECT", CertificationAuth: "f08c47fec0942fa0"},
				{ExchangeDomain: "facebook.com", PublisherID: "123456789", AccountType: "RESELLER"},
			},
			wantErr: false,
		},
		{
			name:    "empty content",
			content: "",
			want:    nil,
			wantErr: true,
			errType: models.ErrInvalidAdsTxtFormat,
		},
		{
			name: "only comments and empty lines",
			content: `# Only comments
# No valid entries
`,
			want:    nil,
			wantErr: true,
			errType: models.ErrInvalidAdsTxtFormat,
		},
		{
			name: "mixed valid and invalid lines",
			content: `google.com, pub-123456789, DIRECT, f08c47fec0942fa0
invalid line without proper format
facebook.com, 123456789, RESELLER`,
			want: []models.AdsTxtEntry{
				{ExchangeDomain: "google.com", PublisherID: "pub-123456789", AccountType: "DIRECT", CertificationAuth: "f08c47fec0942fa0"},
				{ExchangeDomain: "facebook.com", PublisherID: "123456789", AccountType: "RESELLER"},
			},
			wantErr: false,
		},
		{
			name: "case normalization",
			content: `GOOGLE.COM, pub-123456789, direct, f08c47fec0942fa0
Facebook.Com, 123456789, reseller`,
			want: []models.AdsTxtEntry{
				{ExchangeDomain: "google.com", PublisherID: "pub-123456789", AccountType: "DIRECT", CertificationAuth: "f08c47fec0942fa0"},
				{ExchangeDomain: "facebook.com", PublisherID: "123456789", AccountType: "RESELLER"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.Parse(tt.content)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parser.Parse() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("Parser.Parse() error = %v, want error type %v", err, tt.errType)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parser.Parse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_CountAdvertisers(t *testing.T) {
	parser := newParser()
	
	entries := []models.AdsTxtEntry{
		{ExchangeDomain: "google.com", PublisherID: "pub-1", AccountType: "DIRECT"},
		{ExchangeDomain: "google.com", PublisherID: "pub-2", AccountType: "RESELLER"},
		{ExchangeDomain: "facebook.com", PublisherID: "123", AccountType: "DIRECT"},
		{ExchangeDomain: "google.com", PublisherID: "pub-3", AccountType: "DIRECT"},
		{ExchangeDomain: "appnexus.com", PublisherID: "456", AccountType: "RESELLER"},
	}
	
	expected := map[string]int{
		"google.com":   3,
		"facebook.com": 1,
		"appnexus.com": 1,
	}
	
	got := parser.CountAdvertisers(entries)
	
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("Parser.CountAdvertisers() = %v, want %v", got, expected)
	}
}

func TestParser_parseLine(t *testing.T) {
	parser := newParser()
	
	tests := []struct {
		name     string
		line     string
		want     *models.AdsTxtEntry
		wantErr  bool
	}{
		{
			name: "valid line with certification authority",
			line: "google.com, pub-123456789, DIRECT, f08c47fec0942fa0",
			want: &models.AdsTxtEntry{
				ExchangeDomain:    "google.com",
				PublisherID:       "pub-123456789",
				AccountType:       "DIRECT",
				CertificationAuth: "f08c47fec0942fa0",
			},
			wantErr: false,
		},
		{
			name: "valid line without certification authority",
			line: "facebook.com, 123456789, RESELLER",
			want: &models.AdsTxtEntry{
				ExchangeDomain: "facebook.com",
				PublisherID:    "123456789",
				AccountType:    "RESELLER",
			},
			wantErr: false,
		},
		{
			name:    "invalid line - missing fields",
			line:    "google.com, pub-123456789",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid line - wrong format",
			line:    "not a valid ads.txt line",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "invalid domain format",
			line:    "invalid-domain-, pub-123456789, DIRECT",
			want:    nil,
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.parseLine(tt.line)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("Parser.parseLine() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Parser.parseLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parser.parseLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParser_isValidDomain(t *testing.T) {
	parser := newParser()
	
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{"valid domain", "google.com", true},
		{"valid subdomain", "ads.google.com", true},
		{"valid complex domain", "my-site.example-models.co.uk", true},
		{"empty domain", "", false},
		{"domain with invalid characters", "google@com", false},
		{"domain starting with dot", ".google.com", false},
		{"domain ending with dot", "google.com.", true}, // Actually valid
		{"very long domain", string(make([]byte, 260)) + ".com", false},
		{"single character domain", "a", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.isValidDomain(tt.domain)
			if got != tt.want {
				t.Errorf("Parser.isValidDomain() = %v, want %v", got, tt.want)
			}
		})
	}
}