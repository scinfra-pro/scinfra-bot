package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram       TelegramConfig       `yaml:"telegram"`
	Edge           EdgeConfig           `yaml:"edge"`
	Upstreams      map[string]*Upstream `yaml:"upstreams"`
	Webhooks       WebhooksConfig       `yaml:"webhooks"`
	Logging        LoggingConfig        `yaml:"logging"`
	Infrastructure InfrastructureConfig `yaml:"infrastructure"`
	S3             S3Config             `yaml:"s3"`
}

// InfrastructureConfig configures infrastructure monitoring
type InfrastructureConfig struct {
	Enabled       bool          `yaml:"enabled"`
	PrometheusURL string        `yaml:"prometheus_url"`
	Clouds        []CloudConfig `yaml:"clouds"`
}

// CloudConfig represents a cloud provider with servers
type CloudConfig struct {
	Name    string         `yaml:"name"` // "Production"
	Icon    string         `yaml:"icon"` // "☁️"
	Servers []ServerConfig `yaml:"servers"`
}

// ServerConfig represents a server to monitor
type ServerConfig struct {
	ID            string          `yaml:"id"`             // "edge-gateway"
	Name          string          `yaml:"name"`           // "edge-gateway"
	Icon          string          `yaml:"icon"`           // "🖥️"
	IP            string          `yaml:"ip"`             // "10.0.1.11"
	ExternalCheck string          `yaml:"external_check"` // "https://51.250.11.142" or "tcp://..."
	Services      []ServiceConfig `yaml:"services"`
}

// ServiceConfig represents a service running on a server
type ServiceConfig struct {
	Name string `yaml:"name"` // "Nginx"
	Job  string `yaml:"job"`  // Prometheus job name (optional)
	Port int    `yaml:"port"` // Port number (optional, for display)
}

// WebhooksConfig configures the webhook receiver
type WebhooksConfig struct {
	Enabled bool   `yaml:"enabled"`
	Listen  string `yaml:"listen"`
	Secret  string `yaml:"secret"`
}

// Upstream represents a VPS upstream server
type Upstream struct {
	Name           string `yaml:"name"`            // Display name (optional, defaults to key)
	IP             string `yaml:"ip"`
	User           string `yaml:"user"`
	SwitchGate     bool   `yaml:"switch_gate"`
	SwitchGatePort int    `yaml:"switch_gate_port"`
}

type TelegramConfig struct {
	Token          string  `yaml:"token"`
	AllowedChatIDs []int64 `yaml:"allowed_chat_ids"`
}

type EdgeConfig struct {
	Name          string `yaml:"name"`            // Display name for traffic stats
	Host          string `yaml:"host"`
	KeyPath       string `yaml:"key_path"`
	VPNModeScript string `yaml:"vpn_mode_script"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

// Load reads configuration from YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	// Expand environment variables
	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// MergeS3Metadata merges S3 metadata into config
// S3 data takes precedence over YAML for edge, upstreams, and infrastructure
func (c *Config) MergeS3Metadata(metadata *S3Metadata) {
	if metadata == nil {
		return
	}

	// Merge edge config (S3 takes precedence, but keep KeyPath from YAML)
	if metadata.Edge != nil {
		keyPath := c.Edge.KeyPath // preserve from YAML
		c.Edge = *metadata.Edge
		c.Edge.KeyPath = keyPath
	}

	// Merge upstreams (S3 adds to YAML, overwrites by key)
	if len(metadata.Upstreams) > 0 {
		if c.Upstreams == nil {
			c.Upstreams = make(map[string]*Upstream)
		}
		for k, v := range metadata.Upstreams {
			c.Upstreams[k] = v
		}
	}

	// Merge infrastructure clouds (S3 adds to YAML)
	if len(metadata.Clouds) > 0 {
		c.Infrastructure.Clouds = append(c.Infrastructure.Clouds, metadata.Clouds...)
		c.Infrastructure.Enabled = true
	}
}

// ValidateRuntime checks required fields after S3 merge
// Call this after MergeS3Metadata to ensure we have valid config
func (c *Config) ValidateRuntime() error {
	if c.Edge.Host == "" {
		return fmt.Errorf("edge.host is required (configure in YAML or enable S3)")
	}
	if len(c.Upstreams) == 0 {
		return fmt.Errorf("at least one upstream is required (configure in YAML or enable S3)")
	}
	return nil
}

// Validate checks required fields
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if len(c.Telegram.AllowedChatIDs) == 0 {
		return fmt.Errorf("telegram.allowed_chat_ids is required")
	}
	// Edge and upstreams are validated after S3 merge (in ValidateAfterMerge)
	if c.Edge.Name == "" {
		c.Edge.Name = "Edge Gateway"
	}
	if c.Edge.VPNModeScript == "" {
		c.Edge.VPNModeScript = "/usr/local/bin/vpn-mode.sh"
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	// Webhooks defaults
	if c.Webhooks.Listen == "" {
		c.Webhooks.Listen = "0.0.0.0:8080"
	}
	// S3 validation
	if c.S3.Enabled {
		if c.S3.Bucket == "" {
			return fmt.Errorf("s3.bucket is required when s3.enabled is true")
		}
		if c.S3.Endpoint == "" {
			return fmt.Errorf("s3.endpoint is required when s3.enabled is true")
		}
		if c.S3.Region == "" {
			return fmt.Errorf("s3.region is required when s3.enabled is true")
		}
		if len(c.S3.Providers) == 0 {
			return fmt.Errorf("s3.providers is required when s3.enabled is true")
		}
		if c.S3.Prefix == "" {
			c.S3.Prefix = "metadata/"
		}
	}
	// Set defaults for upstreams
	for key, u := range c.Upstreams {
		if u.Name == "" {
			u.Name = capitalize(key)
		}
		if u.User == "" {
			u.User = "root"
		}
		if u.SwitchGatePort == 0 {
			u.SwitchGatePort = 9090
		}
	}

	// Set defaults for infrastructure
	if c.Infrastructure.PrometheusURL == "" {
		c.Infrastructure.PrometheusURL = "http://localhost:9090"
	}
	// Set defaults for servers
	for i := range c.Infrastructure.Clouds {
		cloud := &c.Infrastructure.Clouds[i]
		if cloud.Icon == "" {
			cloud.Icon = "☁️"
		}
		for j := range cloud.Servers {
			server := &cloud.Servers[j]
			if server.Name == "" {
				server.Name = server.ID
			}
			if server.Icon == "" {
				server.Icon = "🖥️"
			}
		}
	}

	return nil
}

// IsValidUpstream checks if upstream name is in the list
func (c *Config) IsValidUpstream(name string) bool {
	_, ok := c.Upstreams[name]
	return ok
}

// GetUpstream returns upstream by name
func (c *Config) GetUpstream(name string) *Upstream {
	if u, ok := c.Upstreams[name]; ok {
		return u
	}
	return nil
}

// GetUpstreamIP returns IP for upstream name
func (c *Config) GetUpstreamIP(name string) string {
	if u, ok := c.Upstreams[name]; ok {
		return u.IP
	}
	return ""
}

// GetUpstreamNames returns list of upstream names
func (c *Config) GetUpstreamNames() []string {
	names := make([]string, 0, len(c.Upstreams))
	for name := range c.Upstreams {
		names = append(names, name)
	}
	return names
}

// IsAllowedChat checks if chat ID is in allowed list
func (c *Config) IsAllowedChat(chatID int64) bool {
	for _, id := range c.Telegram.AllowedChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

// GetUpstreamDisplayName returns display name for upstream
func (c *Config) GetUpstreamDisplayName(name string) string {
	if u, ok := c.Upstreams[name]; ok && u.Name != "" {
		return u.Name
	}
	return capitalize(name)
}

// capitalize returns string with first letter uppercased
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// GetServer returns server config by ID
func (c *Config) GetServer(serverID string) *ServerConfig {
	for i := range c.Infrastructure.Clouds {
		for j := range c.Infrastructure.Clouds[i].Servers {
			if c.Infrastructure.Clouds[i].Servers[j].ID == serverID {
				return &c.Infrastructure.Clouds[i].Servers[j]
			}
		}
	}
	return nil
}

// GetServerCloud returns cloud name for a server
func (c *Config) GetServerCloud(serverID string) string {
	for i := range c.Infrastructure.Clouds {
		for j := range c.Infrastructure.Clouds[i].Servers {
			if c.Infrastructure.Clouds[i].Servers[j].ID == serverID {
				return c.Infrastructure.Clouds[i].Name
			}
		}
	}
	return ""
}

// GetAllServers returns all server configs
func (c *Config) GetAllServers() []ServerConfig {
	var servers []ServerConfig
	for _, cloud := range c.Infrastructure.Clouds {
		servers = append(servers, cloud.Servers...)
	}
	return servers
}

// IsInfrastructureEnabled returns true if infrastructure monitoring is configured
func (c *Config) IsInfrastructureEnabled() bool {
	return c.Infrastructure.Enabled && len(c.Infrastructure.Clouds) > 0
}

// GetUpstreamByIP finds upstream key by IP address
// Returns empty string if not found
func (c *Config) GetUpstreamByIP(ip string) string {
	for key, u := range c.Upstreams {
		if u.IP == ip {
			return key
		}
	}
	return ""
}

// IsSwitchGateServer checks if server has a switch-gate upstream by IP
func (c *Config) IsSwitchGateServer(ip string) bool {
	for _, u := range c.Upstreams {
		if u.IP == ip && u.SwitchGate {
			return true
		}
	}
	return false
}
