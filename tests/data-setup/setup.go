package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Database string `yaml:"database"`
		SSLMode  string `yaml:"ssl_mode"`
	} `yaml:"database"`
}

type Organization struct {
	Name             string `yaml:"name"`
	Description      string `yaml:"description"`
	Address          string `yaml:"address"`
	Metro            string `yaml:"metro"`
	AdminPhoneNumber string `yaml:"admin_phone_number"`
	AdminEmail       string `yaml:"admin_email"`
}

type User struct {
	Email        string `yaml:"email"`
	PhoneNumber  string `yaml:"phone_number"`
	Password     string `yaml:"password"`
	Name         string `yaml:"name"`
	AvatarURL    string `yaml:"avatar_url"`
	Role         string `yaml:"role"`
	Status       string `yaml:"status"`
	BalanceCents int    `yaml:"balance_cents"`
}

type SetupData struct {
	ConfigFile   string       `yaml:"config_file"`
	Organization Organization `yaml:"organization"`
	Users        []User       `yaml:"users"`
}

func main() {
	// Read the setup YAML file
	setupFile := "tests/data-setup/user_org.yaml"

	// Check if file exists, if not try relative path
	if _, err := os.Stat(setupFile); os.IsNotExist(err) {
		setupFile = "user_org.yaml"
	}

	setupData, err := readSetupFile(setupFile)
	if err != nil {
		log.Fatalf("Failed to read setup file: %v", err)
	}

	// Read the config file
	configPath := resolveConfigPath(setupData.ConfigFile)
	config, err := readConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	// Connect to database
	db, err := connectDB(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Populate data
	if err := populateData(db, setupData); err != nil {
		log.Fatalf("Failed to populate data: %v", err)
	}

	log.Println("✅ Test data successfully populated!")
}

func readSetupFile(filename string) (*SetupData, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var setupData SetupData
	if err := yaml.Unmarshal(data, &setupData); err != nil {
		return nil, err
	}

	return &setupData, nil
}

func resolveConfigPath(configPath string) string {
	// Try the path as-is first
	if _, err := os.Stat(configPath); err == nil {
		return configPath
	}

	// Try from project root
	projectRoot := findProjectRoot()
	fullPath := filepath.Join(projectRoot, configPath)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// Return original path and let it fail with a clear error
	return configPath
}

func findProjectRoot() string {
	// Look for go.mod to identify project root
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}

func readConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func connectDB(config *Config) (*sql.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Database.Host,
		config.Database.Port,
		config.Database.User,
		config.Database.Password,
		config.Database.Database,
		config.Database.SSLMode,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("✓ Connected to database: %s@%s:%d/%s",
		config.Database.User,
		config.Database.Host,
		config.Database.Port,
		config.Database.Database)

	return db, nil
}

func populateData(db *sql.DB, data *SetupData) error {
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Create organization
	log.Printf("Creating organization: %s", data.Organization.Name)
	var orgID int32
	err = tx.QueryRow(`
		INSERT INTO orgs (name, description, address, metro, admin_phone_number, admin_email, created_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`,
		data.Organization.Name,
		data.Organization.Description,
		data.Organization.Address,
		data.Organization.Metro,
		data.Organization.AdminPhoneNumber,
		data.Organization.AdminEmail,
		time.Now(),
	).Scan(&orgID)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}
	log.Printf("✓ Organization created with ID: %d", orgID)

	// 2. Create users and link to organization
	for i, user := range data.Users {
		log.Printf("Creating user %d/%d: %s (%s)", i+1, len(data.Users), user.Name, user.Email)

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %w", user.Email, err)
		}

		// Insert user
		var userID int32
		err = tx.QueryRow(`
			INSERT INTO users (email, phone_number, password_hash, name, avatar_url, created_on, updated_on)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`,
			user.Email,
			user.PhoneNumber,
			string(passwordHash),
			user.Name,
			user.AvatarURL,
			time.Now(),
			time.Now(),
		).Scan(&userID)
		if err != nil {
			return fmt.Errorf("failed to create user %s: %w", user.Email, err)
		}

		// Link user to organization
		_, err = tx.Exec(`
			INSERT INTO users_orgs (user_id, org_id, joined_on, balance_cents, status, role)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			userID,
			orgID,
			time.Now(),
			user.BalanceCents,
			user.Status,
			user.Role,
		)
		if err != nil {
			return fmt.Errorf("failed to link user %s to organization: %w", user.Email, err)
		}

		log.Printf("  ✓ User created with ID: %d, Role: %s", userID, user.Role)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
