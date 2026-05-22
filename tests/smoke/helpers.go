package smoke

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"gopkg.in/yaml.v3"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "../../config/config.ec2.apitest.yaml", "smoke test config file")
}

// smokeConfig holds only the fields the smoke tests need.
// gRPC address and TLS mode are derived from server + tls blocks —
// no database credentials required; DB health is verified through the API.
type smokeConfig struct {
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	TLS struct {
		Enabled bool `yaml:"enabled"`
	} `yaml:"tls"`
}

func (c *smokeConfig) grpcAddr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

func loadSmokeConfig(t *testing.T) *smokeConfig {
	t.Helper()
	if !flag.Parsed() {
		flag.Parse()
	}

	finalPath := configPath
	if _, err := os.Stat(finalPath); os.IsNotExist(err) {
		altPath := filepath.Join("..", "..", configPath)
		if _, err2 := os.Stat(altPath); err2 == nil {
			finalPath = altPath
		}
	}

	data, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf(
			"failed to read smoke config %s: %v\n"+
				"  Ensure config/config.ec2.apitest.yaml exists — see deploy/ec2-mvp/docs/handoff.md Phase 2b",
			finalPath, err,
		)
	}

	var cfg smokeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse smoke config %s: %v", finalPath, err)
	}
	if cfg.Server.Host == "" || cfg.Server.Port == 0 {
		t.Fatalf("smoke config %s is missing server.host or server.port", finalPath)
	}
	return &cfg
}

func newGRPCConn(t *testing.T, cfg *smokeConfig) *grpc.ClientConn {
	t.Helper()

	var opt grpc.DialOption
	if cfg.TLS.Enabled {
		// nil cert pool → system trust store (trusts Let's Encrypt natively)
		creds := credentials.NewClientTLSFromCert(nil, "")
		opt = grpc.WithTransportCredentials(creds)
	} else {
		opt = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	conn, err := grpc.NewClient(cfg.grpcAddr(), opt)
	if err != nil {
		t.Fatalf("failed to create gRPC client to %s: %v", cfg.grpcAddr(), err)
	}
	return conn
}
