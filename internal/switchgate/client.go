package switchgate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client provides SSH access to VPS with switch-gate
type Client struct {
	name      string
	jumpHost  string
	targetIP  string
	user      string
	keyPath   string
	apiPort   int
	sshConfig *ssh.ClientConfig
}

// Status represents switch-gate status
type Status struct {
	Mode        string       `json:"mode"`
	ModeHealthy *bool        `json:"mode_healthy,omitempty"` // only with ?check=true
	ModeError   *string      `json:"mode_error,omitempty"`   // only if mode_healthy=false
	Uptime      string       `json:"uptime"`
	Connections int          `json:"connections"`
	Traffic     TrafficStats `json:"traffic"`
	Home        HomeStats    `json:"home"`
	Available   []string     `json:"available_modes"`
}

// TrafficStats represents traffic statistics
type TrafficStats struct {
	DirectMB float64 `json:"direct_mb"`
	WarpMB   float64 `json:"warp_mb"`
	HomeMB   float64 `json:"home_mb"`
	TotalMB  float64 `json:"total_mb"`
}

// HomeStats represents home proxy statistics
type HomeStats struct {
	LimitMB     int     `json:"limit_mb"`
	UsedMB      float64 `json:"used_mb"`
	RemainingMB float64 `json:"remaining_mb"`
	CostUSD     float64 `json:"cost_usd"`
}

// ClientConfig holds configuration for creating a client
type ClientConfig struct {
	Name     string
	JumpHost string // user@host for SSH jump
	TargetIP string // VPS IP address
	User     string // SSH user on VPS
	KeyPath  string // Optional SSH key path
	APIPort  int    // switch-gate API port (default 9090)
}

// NewClient creates a new switch-gate client
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.APIPort == 0 {
		cfg.APIPort = 9090
	}
	if cfg.User == "" {
		cfg.User = "root"
	}

	c := &Client{
		name:     cfg.Name,
		jumpHost: cfg.JumpHost,
		targetIP: cfg.TargetIP,
		user:     cfg.User,
		keyPath:  cfg.KeyPath,
		apiPort:  cfg.APIPort,
	}

	sshConfig, err := c.buildSSHConfig()
	if err != nil {
		return nil, fmt.Errorf("build ssh config: %w", err)
	}
	c.sshConfig = sshConfig

	return c, nil
}

// Name returns the upstream name
func (c *Client) Name() string {
	return c.name
}

// buildSSHConfig creates SSH client configuration
func (c *Client) buildSSHConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH key file first
	if c.keyPath != "" {
		signer, err := c.loadKeyFile(c.keyPath)
		if err != nil {
			return nil, fmt.Errorf("load key file: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	// Try SSH agent
	if agentAuth := c.getAgentAuth(); agentAuth != nil {
		authMethods = append(authMethods, agentAuth)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	return &ssh.ClientConfig{
		User:            c.user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

// loadKeyFile reads SSH private key from file
func (c *Client) loadKeyFile(path string) (ssh.Signer, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}

// getAgentAuth returns SSH agent authentication method
func (c *Client) getAgentAuth() ssh.AuthMethod {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil
	}

	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil
	}

	return ssh.PublicKeysCallback(agent.NewClient(conn).Signers)
}

// exec runs command on VPS via SSH with ProxyJump
func (c *Client) exec(cmd string) (string, error) {
	// Parse jump host
	jumpUser := "master"
	jumpAddr := c.jumpHost
	if idx := strings.Index(c.jumpHost, "@"); idx != -1 {
		jumpUser = c.jumpHost[:idx]
		jumpAddr = c.jumpHost[idx+1:]
	}

	// Connect to jump host
	jumpConfig := &ssh.ClientConfig{
		User:            jumpUser,
		Auth:            c.sshConfig.Auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	jumpConn, err := ssh.Dial("tcp", jumpAddr+":22", jumpConfig)
	if err != nil {
		return "", fmt.Errorf("dial jump host: %w", err)
	}
	defer func() { _ = jumpConn.Close() }()

	// Connect to target through jump host
	targetConn, err := jumpConn.Dial("tcp", c.targetIP+":22")
	if err != nil {
		return "", fmt.Errorf("dial target via jump: %w", err)
	}
	defer func() { _ = targetConn.Close() }()

	// Create SSH connection to target
	ncc, chans, reqs, err := ssh.NewClientConn(targetConn, c.targetIP+":22", c.sshConfig)
	if err != nil {
		return "", fmt.Errorf("ssh client conn: %w", err)
	}
	targetClient := ssh.NewClient(ncc, chans, reqs)
	defer func() { _ = targetClient.Close() }()

	// Create session
	session, err := targetClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Run command
	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("run command: %w (stderr: %s)", err, stderr.String())
	}

	return stdout.String(), nil
}

// GetStatus returns switch-gate status (fast, no health check)
func (c *Client) GetStatus() (*Status, error) {
	cmd := fmt.Sprintf("curl -s http://127.0.0.1:%d/status", c.apiPort)
	output, err := c.exec(cmd)
	if err != nil {
		return nil, err
	}

	var status Status
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	return &status, nil
}

// GetStatusWithCheck returns switch-gate status with mode health check
// This takes ~5 seconds longer due to the connectivity test
func (c *Client) GetStatusWithCheck() (*Status, error) {
	cmd := fmt.Sprintf("curl -s 'http://127.0.0.1:%d/status?check=true'", c.apiPort)
	output, err := c.exec(cmd)
	if err != nil {
		return nil, err
	}

	var status Status
	if err := json.Unmarshal([]byte(output), &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	return &status, nil
}

// SetMode changes switch-gate mode
func (c *Client) SetMode(mode string) error {
	cmd := fmt.Sprintf("curl -s -X POST http://127.0.0.1:%d/mode/%s", c.apiPort, mode)
	output, err := c.exec(cmd)
	if err != nil {
		return err
	}

	// Check response
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if errMsg, ok := resp["error"]; ok {
		return fmt.Errorf("%v", errMsg)
	}

	return nil
}

// GetExternalIP returns current external IP through switch-gate
func (c *Client) GetExternalIP() (string, error) {
	// Use switch-gate SOCKS proxy to get external IP
	cmd := "curl -s -x socks5h://127.0.0.1:18388 --max-time 10 ifconfig.me"
	output, err := c.exec(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// Restart restarts the switch-gate service via systemctl
func (c *Client) Restart() error {
	_, err := c.exec("systemctl restart switch-gate")
	return err
}

// GetModeIcon returns emoji for mode
func GetModeIcon(mode string) string {
	switch strings.ToLower(mode) {
	case "direct":
		return "\U0001F5A5" // 🖥️ Computer - VPS IP
	case "warp":
		return "\u2601\uFE0F" // ☁️ Cloud - Cloudflare
	case "home":
		return "\U0001F3E0" // 🏠 House - Residential
	default:
		return "\u2753" // ❓ Question mark
	}
}
