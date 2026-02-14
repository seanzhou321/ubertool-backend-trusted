package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	SMTP     SMTPConfig     `yaml:"smtp"`
	JWT      JWTConfig      `yaml:"jwt"`
	Storage  StorageConfig  `yaml:"storage"`
	Log       LogConfig       `yaml:"log"`
	Billing   BillingConfig   `yaml:"billing"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

// ServerConfig contains gRPC server settings
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// DatabaseConfig contains PostgreSQL connection settings
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	SSLMode  string `yaml:"ssl_mode"`
}

// SMTPConfig contains email service settings
type SMTPConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	From     string `yaml:"from"`
}

// JWTConfig contains JWT token settings
type JWTConfig struct {
	Secret             string `yaml:"secret"`
	AccessTokenExpiry  int    `yaml:"access_token_expiry_minutes"`
	RefreshTokenExpiry int    `yaml:"refresh_token_expiry_minutes"`
	TempTokenExpiry    int    `yaml:"temp_token_expiry_minutes"`
}

// StorageConfig contains file storage settings
type StorageConfig struct {
	Type         string   `yaml:"type"`       // "mock" or "s3"
	UploadDir    string   `yaml:"upload_dir"` // For mock storage
	BaseURL      string   `yaml:"base_url"`   // Server base URL for mock URLs
	MaxFileSize  int64    `yaml:"max_file_size_mb"`
	AllowedTypes []string `yaml:"allowed_types"`
}

// LogConfig contains logging settings
type LogConfig struct {
	Level  string `yaml:"level"`  // "debug", "info", "warn", "error"
	Format string `yaml:"format"` // "json" or "text"
}

// Load reads configuration from a YAML file
func Load(configPath string) (*Config, error) {
	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables if present
	cfg.overrideWithEnv()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// overrideWithEnv overrides config values with environment variables
func (c *Config) overrideWithEnv() {
	// Database
	if val := os.Getenv("DB_HOST"); val != "" {
		c.Database.Host = val
	}
	if val := os.Getenv("DB_PORT"); val != "" {
		fmt.Sscanf(val, "%d", &c.Database.Port)
	}
	if val := os.Getenv("DB_USER"); val != "" {
		c.Database.User = val
	}
	if val := os.Getenv("DB_PASSWORD"); val != "" {
		c.Database.Password = val
	}
	if val := os.Getenv("DB_NAME"); val != "" {
		c.Database.Database = val
	}
	if val := os.Getenv("DB_SSL_MODE"); val != "" {
		c.Database.SSLMode = val
	}

	// SMTP
	if val := os.Getenv("SMTP_HOST"); val != "" {
		c.SMTP.Host = val
	}
	if val := os.Getenv("SMTP_PORT"); val != "" {
		fmt.Sscanf(val, "%d", &c.SMTP.Port)
	}
	if val := os.Getenv("SMTP_USER"); val != "" {
		c.SMTP.User = val
	}
	if val := os.Getenv("SMTP_PASSWORD"); val != "" {
		c.SMTP.Password = val
	}
	if val := os.Getenv("SMTP_FROM"); val != "" {
		c.SMTP.From = val
	}

	// JWT
	if val := os.Getenv("JWT_SECRET"); val != "" {
		c.JWT.Secret = val
	}

	// Server
	if val := os.Getenv("SERVER_HOST"); val != "" {
		c.Server.Host = val
	}
	if val := os.Getenv("SERVER_PORT"); val != "" {
		fmt.Sscanf(val, "%d", &c.Server.Port)
	}

	// Storage
	if val := os.Getenv("UPLOAD_DIR"); val != "" {
		c.Storage.UploadDir = val
	}

	// Log
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		c.Log.Level = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		c.Log.Format = val
	}

	// Billing
	if val := os.Getenv("BILLING_THRESHOLD_CENTS"); val != "" {
		fmt.Sscanf(val, "%d", &c.Billing.SettlementThresholdCents)
	}

	// Set defaults for log if not configured
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.Format == "" {
		c.Log.Format = "text"
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Server validation
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Database validation
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// SMTP validation
	if c.SMTP.Host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if c.SMTP.Port <= 0 || c.SMTP.Port > 65535 {
		return fmt.Errorf("invalid SMTP port: %d", c.SMTP.Port)
	}

	// JWT validation
	if c.JWT.Secret == "" {
		return fmt.Errorf("JWT secret is required")
	}
	if len(c.JWT.Secret) < 32 {
		return fmt.Errorf("JWT secret must be at least 32 characters")
	}

	// Storage validation
	if c.Storage.UploadDir == "" {
		return fmt.Errorf("upload directory is required")
	}

	// Billing defaults
	if c.Billing.SettlementThresholdCents == 0 {
		c.Billing.SettlementThresholdCents = 500 // Default $5.00
	}

	// Scheduler defaults
	if c.Scheduler.MarkOverdueRentals == "" {
		c.Scheduler.MarkOverdueRentals = "0 0 2 * * *" // 2 AM UTC
	}
	if c.Scheduler.SendOverdueReminders == "" {
		c.Scheduler.SendOverdueReminders = "0 0 3 * * *" // 3 AM UTC
	}
	if c.Scheduler.SendBillReminders == "" {
		c.Scheduler.SendBillReminders = "0 0 4 * * *" // 4 AM UTC
	}
	if c.Scheduler.CheckOverdueBills == "" {
		c.Scheduler.CheckOverdueBills = "0 0 5 10 * *" // 10th of month at 5 AM UTC
	}
	if c.Scheduler.ResolveDisputedBills == "" {
		c.Scheduler.ResolveDisputedBills = "0 0 23 L * *" // Last day of month at 11 PM UTC
	}
	if c.Scheduler.TakeBalanceSnapshots == "" {
		c.Scheduler.TakeBalanceSnapshots = "0 30 23 L * *" // Last day of month at 11:30 PM UTC
	}
	if c.Scheduler.PerformBillSplitting == "" {
		c.Scheduler.PerformBillSplitting = "0 0 0 1 * *" // 1st of month at 12 AM UTC
	}
	if c.Scheduler.SendBillNotices == "" {
		c.Scheduler.SendBillNotices = "0 0 9 * * *" // Daily at 9 AM UTC
	}

	return nil
}

// GetDatabaseConnectionString returns a PostgreSQL connection string
func (c *Config) GetDatabaseConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
		c.Database.SSLMode,
	)
}

// GetServerAddress returns the gRPC server address
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// BillingConfig contains bill splitting settings
type BillingConfig struct {
	SettlementThresholdCents int `yaml:"settlement_threshold_cents"`
}

// SchedulerConfig contains cron schedule settings
type SchedulerConfig struct {
	MarkOverdueRentals   string `yaml:"mark_overdue_rentals"`
	SendOverdueReminders string `yaml:"send_overdue_reminders"`
	SendBillReminders    string `yaml:"send_bill_reminders"`
	CheckOverdueBills    string `yaml:"check_overdue_bills"`
	ResolveDisputedBills string `yaml:"resolve_disputed_bills"`
	TakeBalanceSnapshots string `yaml:"take_balance_snapshots"`
	PerformBillSplitting string `yaml:"perform_bill_splitting"`
	SendBillNotices      string `yaml:"send_bill_notices"`
}
