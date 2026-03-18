package config

import (
	"fmt"
	"os"
	"path/filepath"

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

	data, err := os.ReadFile(absPath)
	if err != nil {
		panic(fmt.Errorf("read config file %s: %w", absPath, err))
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(fmt.Errorf("unmarshal config file %s: %w", absPath, err))
	}

	return &cfg
}
