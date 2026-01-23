package storage

// Config holds storage configuration
type Config struct {
	Type                string // "mock" or "s3"
	MockDir             string // Directory for mock storage
	BaseURL             string // Server base URL for generating mock URLs
	PresignedExpiration string // e.g., "15m"
}
