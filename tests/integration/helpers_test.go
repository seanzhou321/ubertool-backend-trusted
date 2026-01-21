package integration

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ubertool-backend-trusted/internal/config"

	_ "github.com/lib/pq"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "../../config/config.test.yaml", "path to config file")
}

func prepareDB(t *testing.T) *sql.DB {
	// Ensure flags are parsed
	if !flag.Parsed() {
		flag.Parse()
	}

	// Logic to handle running from root vs package dir
	finalPath := configPath
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		// If running from tests/integration, try going up
		altPath := filepath.Join("..", "..", configPath)
		if _, err := os.Stat(altPath); err == nil {
			finalPath = altPath
		}
	}

	cfg, err := config.Load(finalPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", finalPath, err)
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database,
		cfg.Database.SSLMode,
	)

	var db *sql.DB

	// Retry connection as DB might still be starting up
	for i := 0; i < 10; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				return db
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("failed to connect to database: %v", err)
	return nil
}
