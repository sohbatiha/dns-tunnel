package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for the local DNS server
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	API      APIConfig      `yaml:"api"`
	Cache    CacheConfig    `yaml:"cache"`
	Security SecurityConfig `yaml:"security"`
	Logging  LoggingConfig  `yaml:"logging"`
}

// ServerConfig holds DNS server settings
type ServerConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	Port       int    `yaml:"port"`
	Protocol   string `yaml:"protocol"` // udp, tcp, both
}

// APIConfig holds remote API settings
type APIConfig struct {
	Endpoints       []EndpointConfig `yaml:"endpoints"`
	Timeout         time.Duration    `yaml:"timeout"`
	MaxRetries      int              `yaml:"max_retries"`
	RetryDelay      time.Duration    `yaml:"retry_delay"`
	HealthCheckFreq time.Duration    `yaml:"health_check_freq"`
	LoadBalancing   string           `yaml:"load_balancing"` // round_robin, random, failover
}

// EndpointConfig holds configuration for a single API endpoint
type EndpointConfig struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
	Weight int    `yaml:"weight"` // For weighted load balancing
}

// CacheConfig holds DNS cache settings
type CacheConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxItems    int           `yaml:"max_items"`
	DefaultTTL  time.Duration `yaml:"default_ttl"`
	MinTTL      time.Duration `yaml:"min_ttl"`
	MaxTTL      time.Duration `yaml:"max_ttl"`
	NegativeTTL time.Duration `yaml:"negative_ttl"` // For NXDOMAIN caching
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	EncryptionEnabled bool   `yaml:"encryption_enabled"`
	EncryptionKey     string `yaml:"encryption_key"` // 32 bytes hex for AES-256
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	OutputFile string `yaml:"output_file"`
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

	cfg.setDefaults()

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

func (c *Config) setDefaults() {
	if c.Server.ListenAddr == "" {
		c.Server.ListenAddr = "127.0.0.1"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 53
	}
	if c.Server.Protocol == "" {
		c.Server.Protocol = "udp"
	}
	if c.API.Timeout == 0 {
		c.API.Timeout = 10 * time.Second
	}
	if c.API.MaxRetries == 0 {
		c.API.MaxRetries = 3
	}
	if c.API.RetryDelay == 0 {
		c.API.RetryDelay = 500 * time.Millisecond
	}
	if c.API.HealthCheckFreq == 0 {
		c.API.HealthCheckFreq = 30 * time.Second
	}
	if c.API.LoadBalancing == "" {
		c.API.LoadBalancing = "round_robin"
	}
	if c.Cache.MaxItems == 0 {
		c.Cache.MaxItems = 10000
	}
	if c.Cache.DefaultTTL == 0 {
		c.Cache.DefaultTTL = 5 * time.Minute
	}
	if c.Cache.MinTTL == 0 {
		c.Cache.MinTTL = 60 * time.Second
	}
	if c.Cache.MaxTTL == 0 {
		c.Cache.MaxTTL = 24 * time.Hour
	}
	if c.Cache.NegativeTTL == 0 {
		c.Cache.NegativeTTL = 5 * time.Minute
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
}

func (c *Config) validate() error {
	if len(c.API.Endpoints) == 0 {
		return fmt.Errorf("at least one API endpoint is required")
	}
	for i, ep := range c.API.Endpoints {
		if ep.URL == "" {
			return fmt.Errorf("endpoint %d: URL is required", i)
		}
		if ep.APIKey == "" {
			return fmt.Errorf("endpoint %d: API key is required", i)
		}
	}
	if c.Security.EncryptionEnabled && len(c.Security.EncryptionKey) != 64 {
		return fmt.Errorf("encryption key must be 64 hex characters (32 bytes)")
	}
	return nil
}
