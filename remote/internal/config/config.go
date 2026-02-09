package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the remote DNS API server
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Resolver ResolverConfig `yaml:"resolver"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds HTTP server settings
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	TLSCertFile  string        `yaml:"tls_cert_file"`
	TLSKeyFile   string        `yaml:"tls_key_file"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// ResolverConfig holds DNS resolver settings
type ResolverConfig struct {
	Upstreams     []string      `yaml:"upstreams"`
	Timeout       time.Duration `yaml:"timeout"`
	MaxRetries    int           `yaml:"max_retries"`
	CacheEnabled  bool          `yaml:"cache_enabled"`
	CacheTTL      time.Duration `yaml:"cache_ttl"`
	CacheMaxItems int           `yaml:"cache_max_items"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	APIKeys           []string `yaml:"api_keys"`
	EncryptionEnabled bool     `yaml:"encryption_enabled"`
	EncryptionKey     string   `yaml:"encryption_key"` // 32 bytes hex for AES-256
	RateLimitEnabled  bool     `yaml:"rate_limit_enabled"`
	RateLimitPerSec   float64  `yaml:"rate_limit_per_sec"`
	RateLimitBurst    int      `yaml:"rate_limit_burst"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `yaml:"level"` // debug, info, warn, error
	Format     string `yaml:"format"` // json, text
	OutputFile string `yaml:"output_file"` // empty for stdout
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	// Validate
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.Host == "" {
		c.Server.Host = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8443
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 30 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 30 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 120 * time.Second
	}
	if len(c.Resolver.Upstreams) == 0 {
		c.Resolver.Upstreams = []string{"8.8.8.8:53", "1.1.1.1:53", "8.8.4.4:53"}
	}
	if c.Resolver.Timeout == 0 {
		c.Resolver.Timeout = 5 * time.Second
	}
	if c.Resolver.MaxRetries == 0 {
		c.Resolver.MaxRetries = 3
	}
	if c.Resolver.CacheTTL == 0 {
		c.Resolver.CacheTTL = 5 * time.Minute
	}
	if c.Resolver.CacheMaxItems == 0 {
		c.Resolver.CacheMaxItems = 10000
	}
	if c.Security.RateLimitPerSec == 0 {
		c.Security.RateLimitPerSec = 100
	}
	if c.Security.RateLimitBurst == 0 {
		c.Security.RateLimitBurst = 200
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}
}

func (c *Config) validate() error {
	if len(c.Security.APIKeys) == 0 {
		return fmt.Errorf("at least one API key is required")
	}
	if c.Security.EncryptionEnabled && len(c.Security.EncryptionKey) != 64 {
		return fmt.Errorf("encryption key must be 64 hex characters (32 bytes)")
	}
	return nil
}
