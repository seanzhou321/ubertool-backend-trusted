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
	Secret              string `yaml:"secret"`
	AccessTokenExpiry   int    `yaml:"access_token_expiry_minutes"`
	RefreshTokenExpiry  int    `yaml:"refresh_token_expiry_minutes"`
	TempTokenExpiry     int    `yaml:"temp_token_expiry_minutes"`
}

// StorageConfig contains file storage settings
type StorageConfig struct {
	UploadDir     string `yaml:"upload_dir"`
	MaxFileSize   int64  `yaml:"max_file_size_mb"`
	AllowedTypes  []string `yaml:"allowed_types"`
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
