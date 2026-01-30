package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram  TelegramConfig       `yaml:"telegram"`
	Edge      EdgeConfig           `yaml:"edge"`
	Upstreams map[string]*Upstream `yaml:"upstreams"`
	Webhooks  WebhooksConfig       `yaml:"webhooks"`
	Logging   LoggingConfig        `yaml:"logging"`
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

// Validate checks required fields
func (c *Config) Validate() error {
	if c.Telegram.Token == "" {
		return fmt.Errorf("telegram.token is required")
	}
	if len(c.Telegram.AllowedChatIDs) == 0 {
		return fmt.Errorf("telegram.allowed_chat_ids is required")
	}
	if c.Edge.Host == "" {
		return fmt.Errorf("edge.host is required")
	}
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
	// Require at least one upstream
	if len(c.Upstreams) == 0 {
		return fmt.Errorf("at least one upstream is required in config")
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
