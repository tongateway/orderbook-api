package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	TLS      bool   `yaml:"tls"`
}

type NetworkConfig struct {
	Mainnet bool `yaml:"mainnet"`
}

type ApiConfig struct {
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	RPS          int    `yaml:"rps"`
	AuthRequired bool   `yaml:"auth_required"` // Whether API key authentication is required (keys are stored in database)
}

type DatabaseConfig struct {
	Type     string `yaml:"type"`     // sqlite, postgres, mysql
	DSN      string `yaml:"dsn"`      // Data Source Name (path для sqlite, connection string для других)
	Host     string `yaml:"host"`     // Database host (for postgres/mysql)
	Port     string `yaml:"port"`     // Database port (for postgres/mysql)
	User     string `yaml:"user"`     // Database user (for postgres/mysql)
	Password string `yaml:"password"` // Database password (for postgres/mysql)
	DBName   string `yaml:"dbname"`   // Database name (for postgres/mysql)
	SSLMode  string `yaml:"sslmode"`  // SSL mode (for postgres: disable, require, verify-ca, verify-full)
}

type Config struct {
	Redis    RedisConfig    `yaml:"redis"`
	Network  NetworkConfig  `yaml:"network"`
	Api      ApiConfig      `yaml:"api"`
	Database DatabaseConfig `yaml:"database"`
}

const defaultConfigPath = "configs/config.yaml"

// InitConfig reads configuration from config.yaml (or CONFIG_PATH override)
// and returns the parsed Config. Panics if the configuration cannot be loaded.
func InitConfig() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		panic(fmt.Errorf("resolve config path: %w", err))
	}

	// Audit 05-C1: only allow CONFIG_PATH within whitelisted directories.
	// Without this, an attacker who can set the env var (CI leak,
	// container misconfig, operator error) could read arbitrary files
	// like /etc/shadow or /proc/self/environ via subsequent error
	// messages.
	if !isAllowedConfigPath(absPath) {
		panic(fmt.Errorf("config path not in allowed directory: %s", absPath))
	}

	data, err := os.ReadFile(absPath) //nolint:gosec // path validated by isAllowedConfigPath
	if err != nil {
		// Don't echo file content / arbitrary substrings on error.
		panic(fmt.Errorf("read config file: %w", err))
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		// Same — don't reveal file substrings.
		panic(fmt.Errorf("unmarshal config (path: %s): yaml error", absPath))
	}

	return &cfg
}

// isAllowedConfigPath restricts CONFIG_PATH to safe locations.
func isAllowedConfigPath(absPath string) bool {
	allowed := []string{
		"/etc/open4dev-api/",
		"/app/configs/",
		"/srv/configs/",
	}
	// Allow current working dir for local dev (matches `./configs/...`).
	if cwd, err := os.Getwd(); err == nil {
		allowed = append(allowed, cwd+"/")
	}
	for _, prefix := range allowed {
		if strings.HasPrefix(absPath, prefix) {
			return true
		}
	}
	return false
}
