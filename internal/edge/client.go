package edge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// Client provides SSH access to edge-gateway
type Client struct {
	host          string
	keyPath       string
	vpnModeScript string
	sshConfig     *ssh.ClientConfig

	// SSH statistics (in-memory, resets on restart)
	sshSuccessCount int
	sshErrorCount   int
	sshLastLatency  time.Duration
	sshLastError    string
	sshLastErrorAt  time.Time
	sshMu           sync.Mutex
}

// SSHStats holds SSH connection statistics
type SSHStats struct {
	SuccessCount int
	ErrorCount   int
	LastLatency  time.Duration
	LastError    string
	LastErrorAt  time.Time
}

// Status represents edge-gateway VPN status
type Status struct {
	Server string `json:"server"`
	Mode   string `json:"mode"`
	Table  string `json:"table"`
}

// TrafficStats represents edge gateway traffic statistics
type TrafficStats struct {
	Timestamp  string                      `json:"timestamp"`
	Interfaces map[string]InterfaceTraffic `json:"interfaces"`
	Summary    TrafficSummary              `json:"summary"`
	Billing    TrafficBilling              `json:"billing"`
}

// InterfaceTraffic represents traffic for a single interface
type InterfaceTraffic struct {
	Name    string  `json:"name"`
	TxBytes int64   `json:"tx_bytes"`
	TxMB    float64 `json:"tx_mb"`
}

// TrafficSummary represents summarized traffic
type TrafficSummary struct {
	DirectMB float64 `json:"direct_mb"`
	VpnMB    float64 `json:"vpn_mb"`
	TotalMB  float64 `json:"total_mb"`
	TotalGB  float64 `json:"total_gb"`
}

// TrafficBilling represents billing information
type TrafficBilling struct {
	FreeQuotaGB  float64 `json:"free_quota_gb"`
	BillableGB   float64 `json:"billable_gb"`
	RateRubPerGB float64 `json:"rate_rub_per_gb"`
	CostRub      float64 `json:"cost_rub"`
}

// New creates a new edge client
func New(host, keyPath, vpnModeScript string) (*Client, error) {
	c := &Client{
		host:          host,
		keyPath:       keyPath,
		vpnModeScript: vpnModeScript,
	}

	sshConfig, err := c.buildSSHConfig()
	if err != nil {
		return nil, fmt.Errorf("build ssh config: %w", err)
	}
	c.sshConfig = sshConfig

	return c, nil
}

// GetSSHStats returns SSH connection statistics
func (c *Client) GetSSHStats() SSHStats {
	c.sshMu.Lock()
	defer c.sshMu.Unlock()

	return SSHStats{
		SuccessCount: c.sshSuccessCount,
		ErrorCount:   c.sshErrorCount,
		LastLatency:  c.sshLastLatency,
		LastError:    c.sshLastError,
		LastErrorAt:  c.sshLastErrorAt,
	}
}

// recordSSHResult records the result of an SSH operation
func (c *Client) recordSSHResult(err error, latency time.Duration) {
	c.sshMu.Lock()
	defer c.sshMu.Unlock()

	c.sshLastLatency = latency

	if err != nil {
		c.sshErrorCount++
		c.sshLastError = err.Error()
		c.sshLastErrorAt = time.Now()
	} else {
		c.sshSuccessCount++
	}
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

	// Parse user@host
	user := "root"
	host := c.host
	if idx := strings.Index(c.host, "@"); idx != -1 {
		user = c.host[:idx]
		host = c.host[idx+1:]
	}
	c.host = host

	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: use known_hosts
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

// exec runs command on edge-gateway via SSH
func (c *Client) exec(cmd string) (string, error) {
	start := time.Now()

	result, err := c.execInternal(cmd)

	// Record statistics
	latency := time.Since(start)
	c.recordSSHResult(err, latency)

	return result, err
}

// execInternal performs the actual SSH command execution
func (c *Client) execInternal(cmd string) (string, error) {
	// Connect
	conn, err := ssh.Dial("tcp", c.host+":22", c.sshConfig)
	if err != nil {
		return "", fmt.Errorf("ssh dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Create session
	session, err := conn.NewSession()
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

// GetStatus returns current VPN status
func (c *Client) GetStatus() (*Status, error) {
	output, err := c.exec(c.vpnModeScript + " status")
	if err != nil {
		return nil, err
	}
	return c.parseStatus(output)
}

// SetMode changes VPN mode
func (c *Client) SetMode(mode string) error {
	cmd := fmt.Sprintf("sudo %s mode %s", c.vpnModeScript, mode)
	_, err := c.exec(cmd)
	return err
}

// SetModeWithParams changes VPN mode with table
func (c *Client) SetModeWithParams(mode, table string) error {
	cmd := fmt.Sprintf("sudo %s mode %s %s", c.vpnModeScript, mode, table)
	_, err := c.exec(cmd)
	return err
}

// SetUpstream changes upstream server
func (c *Client) SetUpstream(name string) error {
	cmd := fmt.Sprintf("sudo %s upstream %s", c.vpnModeScript, name)
	_, err := c.exec(cmd)
	return err
}

// GetExternalIP returns current external IP
func (c *Client) GetExternalIP() (string, error) {
	output, err := c.exec("curl -s --max-time 5 api.ipify.org")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// parseStatus parses vpn-mode.sh status output
func (c *Client) parseStatus(output string) (*Status, error) {
	status := &Status{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SERVER") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.Server = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "MODE") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.Mode = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "TABLE") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				status.Table = strings.TrimSpace(parts[1])
			}
		}
	}

	return status, nil
}

// GetTraffic returns edge gateway traffic statistics
func (c *Client) GetTraffic() (*TrafficStats, error) {
	output, err := c.exec("/usr/local/bin/yc-traffic.sh")
	if err != nil {
		return nil, fmt.Errorf("get traffic: %w", err)
	}

	var stats TrafficStats
	if err := json.Unmarshal([]byte(output), &stats); err != nil {
		return nil, fmt.Errorf("parse traffic: %w", err)
	}

	return &stats, nil
}
