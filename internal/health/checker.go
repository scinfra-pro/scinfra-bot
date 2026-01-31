package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/scinfra-pro/scinfra-bot/internal/config"
	"github.com/scinfra-pro/scinfra-bot/internal/prometheus"
	"github.com/scinfra-pro/scinfra-bot/internal/switchgate"
)

// ServerStatus represents the health status of a server
type ServerStatus struct {
	ID        string // "edge-gateway"
	Name      string // "edge-gateway"
	Icon      string // "🖥️"
	CloudName string // "Production"
	CloudIcon string // "☁️"
	IP        string // "10.0.1.11"

	// Prometheus data
	IsUp   bool    // up == 1
	CPU    float64 // 0-100%
	Memory float64 // 0-100%
	Disk   float64 // 0-100%

	// Memory details (for display)
	MemoryUsedGB  float64
	MemoryTotalGB float64

	// Disk details (for display)
	DiskUsedGB  float64
	DiskTotalGB float64

	// Uptime
	Uptime time.Duration

	// External accessibility check
	ExternalAccess  bool          // HTTPS/TCP probe succeeded
	ExternalLatency time.Duration // Response time
	ExternalError   string        // Error message if failed

	// Services status
	Services []ServiceStatus
}

// ServiceStatus represents the health status of a service
type ServiceStatus struct {
	Name  string // "Nginx"
	Job   string // Prometheus job name
	Port  int    // Port number
	IsUp  bool   // Service is running
	Error string // Error if failed
}

// StatusLevel represents the health level
type StatusLevel string

const (
	StatusUp       StatusLevel = "up"       // 🟢
	StatusDegraded StatusLevel = "degraded" // 🟡
	StatusDown     StatusLevel = "down"     // 🛑
)

// Checker performs health checks on infrastructure
type Checker struct {
	prometheus        *prometheus.Client
	config            *config.Config
	httpClient        *http.Client
	switchGateClients map[string]*switchgate.Client // key is upstream name (e.g., "primary")

	// Cache
	cache     map[string]*ServerStatus // serverID -> status
	cacheTime time.Time
	cacheTTL  time.Duration
}

// DefaultCacheTTL is the default cache time-to-live
const DefaultCacheTTL = 60 * time.Second

// NewChecker creates a new health checker
func NewChecker(cfg *config.Config, sgClients map[string]*switchgate.Client) *Checker {
	promClient := prometheus.NewClient(cfg.Infrastructure.PrometheusURL)

	return &Checker{
		prometheus:        promClient,
		config:            cfg,
		switchGateClients: sgClients,
		cache:             make(map[string]*ServerStatus),
		cacheTTL:          DefaultCacheTTL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Accept self-signed certs for health checks
				},
			},
		},
	}
}

// CheckAll checks all configured servers (uses cache if valid)
func (c *Checker) CheckAll() ([]*ServerStatus, error) {
	// Return from cache if still valid
	if c.isCacheValid() {
		return c.getCachedStatuses(), nil
	}

	return c.refreshAll()
}

// CheckAllForce forces a refresh bypassing cache
func (c *Checker) CheckAllForce() ([]*ServerStatus, error) {
	return c.refreshAll()
}

// refreshAll fetches fresh data and updates cache
func (c *Checker) refreshAll() ([]*ServerStatus, error) {
	var statuses []*ServerStatus

	for _, cloud := range c.config.Infrastructure.Clouds {
		for _, server := range cloud.Servers {
			status := c.checkServer(&server, cloud.Name, cloud.Icon)
			statuses = append(statuses, status)
			// Update cache
			c.cache[server.ID] = status
		}
	}

	c.cacheTime = time.Now()
	return statuses, nil
}

// CheckServer checks a single server by ID (uses cache if valid)
func (c *Checker) CheckServer(serverID string) (*ServerStatus, error) {
	// Return from cache if valid
	if c.isCacheValid() {
		if status, ok := c.cache[serverID]; ok {
			return status, nil
		}
	}

	return c.checkServerForce(serverID)
}

// CheckServerForce forces a refresh for a single server
func (c *Checker) CheckServerForce(serverID string) (*ServerStatus, error) {
	return c.checkServerForce(serverID)
}

// checkServerForce fetches fresh data for a server
func (c *Checker) checkServerForce(serverID string) (*ServerStatus, error) {
	server := c.config.GetServer(serverID)
	if server == nil {
		return nil, fmt.Errorf("server not found: %s", serverID)
	}

	cloudName := c.config.GetServerCloud(serverID)
	cloudIcon := "☁️"
	for _, cloud := range c.config.Infrastructure.Clouds {
		if cloud.Name == cloudName {
			cloudIcon = cloud.Icon
			break
		}
	}

	return c.checkServer(server, cloudName, cloudIcon), nil
}

// checkServer performs all health checks for a server
func (c *Checker) checkServer(server *config.ServerConfig, cloudName, cloudIcon string) *ServerStatus {
	status := &ServerStatus{
		ID:        server.ID,
		Name:      server.Name,
		Icon:      server.Icon,
		CloudName: cloudName,
		CloudIcon: cloudIcon,
		IP:        server.IP,
	}

	// Check if this is a switch-gate server (remote VPS)
	upstreamKey := c.config.GetUpstreamByIP(server.IP)
	if upstreamKey != "" && c.config.IsSwitchGateServer(server.IP) {
		// Use switch-gate client for remote VPS
		c.checkSwitchGateServer(status, server, upstreamKey)
	} else {
		// Use Prometheus for local/cloud servers
		c.checkPrometheusServer(status, server)
	}

	// Check external accessibility
	if server.ExternalCheck != "" {
		accessible, latency, err := c.checkExternal(server.ExternalCheck)
		status.ExternalAccess = accessible
		status.ExternalLatency = latency
		if err != nil {
			status.ExternalError = err.Error()
		}
	} else {
		// No external check configured - mark as N/A (accessible if up)
		status.ExternalAccess = status.IsUp
	}

	return status
}

// checkPrometheusServer checks a server using Prometheus metrics
func (c *Checker) checkPrometheusServer(status *ServerStatus, server *config.ServerConfig) {
	// Use PrometheusInstance for queries (matches instance label in Prometheus config)
	promInstance := server.PrometheusInstance
	if promInstance == "" {
		promInstance = server.Name // Fallback to name if not set
	}

	// Check if server is up via Prometheus
	isUp, err := c.prometheus.IsUp(promInstance)
	if err != nil {
		status.IsUp = false
	} else {
		status.IsUp = isUp
	}

	// Get metrics only if server is up
	if status.IsUp {
		// CPU
		if cpu, err := c.prometheus.GetCPU(promInstance); err == nil {
			status.CPU = cpu
		}

		// Memory
		if mem, err := c.prometheus.GetMemory(promInstance); err == nil {
			status.Memory = mem
		}
		if used, total, err := c.prometheus.GetMemoryBytes(promInstance); err == nil {
			status.MemoryUsedGB = used / (1024 * 1024 * 1024)
			status.MemoryTotalGB = total / (1024 * 1024 * 1024)
		}

		// Disk
		if disk, err := c.prometheus.GetDisk(promInstance); err == nil {
			status.Disk = disk
		}
		if used, total, err := c.prometheus.GetDiskBytes(promInstance); err == nil {
			status.DiskUsedGB = used / (1024 * 1024 * 1024)
			status.DiskTotalGB = total / (1024 * 1024 * 1024)
		}

		// Uptime
		if uptime, err := c.prometheus.GetUptime(promInstance); err == nil {
			status.Uptime = uptime
		}
	}

	// Check services via Prometheus
	for _, svc := range server.Services {
		svcStatus := ServiceStatus{
			Name: svc.Name,
			Job:  svc.Job,
			Port: svc.Port,
		}

		if svc.Job != "" {
			// Check via Prometheus job
			isUp, err := c.prometheus.IsServiceUp(svc.Job, promInstance)
			svcStatus.IsUp = isUp
			if err != nil {
				svcStatus.Error = err.Error()
			}
		} else {
			// If no job specified, assume running if server is up
			svcStatus.IsUp = status.IsUp
		}

		status.Services = append(status.Services, svcStatus)
	}
}

// checkSwitchGateServer checks a remote VPS using switch-gate API via SSH
func (c *Checker) checkSwitchGateServer(status *ServerStatus, server *config.ServerConfig, upstreamKey string) {
	sgClient, ok := c.switchGateClients[upstreamKey]
	if !ok {
		// No switch-gate client available
		status.IsUp = false
		return
	}

	// Get status from switch-gate API
	sgStatus, err := sgClient.GetStatus()
	if err != nil {
		status.IsUp = false
		// Add error as service status
		status.Services = append(status.Services, ServiceStatus{
			Name:  "switch-gate",
			Port:  9090,
			IsUp:  false,
			Error: err.Error(),
		})
		return
	}

	// Server is up if we got a response
	status.IsUp = true

	// Parse uptime from switch-gate status
	if sgStatus.Uptime != "" {
		if uptime, err := time.ParseDuration(sgStatus.Uptime); err == nil {
			status.Uptime = uptime
		}
	}

	// Get system metrics from node_exporter
	if nodeMetrics, err := sgClient.GetNodeMetrics(); err == nil {
		// Memory
		status.Memory = nodeMetrics.MemoryUsedPercent
		status.MemoryUsedGB = nodeMetrics.MemoryUsedBytes / (1024 * 1024 * 1024)
		status.MemoryTotalGB = nodeMetrics.MemoryTotalBytes / (1024 * 1024 * 1024)

		// Disk
		status.Disk = nodeMetrics.DiskUsedPercent
		status.DiskUsedGB = nodeMetrics.DiskUsedBytes / (1024 * 1024 * 1024)
		status.DiskTotalGB = nodeMetrics.DiskTotalBytes / (1024 * 1024 * 1024)

		// CPU - use load1 as percentage approximation (rough, but useful)
		// For single-core VPS, load1 of 1.0 = 100% CPU
		// We cap at 100% for display purposes
		status.CPU = nodeMetrics.Load1 * 100
		if status.CPU > 100 {
			status.CPU = 100
		}
	}

	// Add services based on config and switch-gate response
	for _, svc := range server.Services {
		svcStatus := ServiceStatus{
			Name: svc.Name,
			Port: svc.Port,
		}

		switch svc.Name {
		case "switch-gate":
			// switch-gate is up if we got status
			svcStatus.IsUp = true
		case "gost":
			// gost is up if switch-gate is running (they're related)
			svcStatus.IsUp = true
		case "node_exporter":
			// node_exporter is up if we got metrics
			svcStatus.IsUp = status.Memory > 0
		default:
			// Other services - assume up if server responds
			svcStatus.IsUp = true
		}

		status.Services = append(status.Services, svcStatus)
	}
}

// checkExternal performs an external accessibility check
func (c *Checker) checkExternal(checkURL string) (bool, time.Duration, error) {
	start := time.Now()

	// Parse check type
	if strings.HasPrefix(checkURL, "tcp://") {
		// TCP check
		addr := strings.TrimPrefix(checkURL, "tcp://")
		return c.checkTCP(addr, start)
	}

	// Default: HTTPS/HTTP check
	return c.checkHTTP(checkURL, start)
}

// checkHTTP performs an HTTP/HTTPS check
func (c *Checker) checkHTTP(url string, start time.Time) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, 0, fmt.Errorf("invalid URL: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		return false, latency, fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Consider 2xx and 3xx as success
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, latency, nil
	}

	return false, latency, fmt.Errorf("HTTP %d", resp.StatusCode)
}

// checkTCP performs a TCP connection check
func (c *Checker) checkTCP(addr string, start time.Time) (bool, time.Duration, error) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	latency := time.Since(start)

	if err != nil {
		return false, latency, fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = conn.Close() }()

	return true, latency, nil
}

// GetStatusLevel returns the status level for a server
func (s *ServerStatus) GetStatusLevel() StatusLevel {
	if !s.IsUp {
		return StatusDown
	}

	// Check for degraded conditions
	if s.CPU > 80 || s.Memory > 85 || s.Disk > 85 {
		return StatusDegraded
	}

	// Check if any service is down
	for _, svc := range s.Services {
		if !svc.IsUp {
			return StatusDegraded
		}
	}

	return StatusUp
}

// GetStatusIcon returns the status icon for a server
func (s *ServerStatus) GetStatusIcon() string {
	switch s.GetStatusLevel() {
	case StatusUp:
		return "🟢"
	case StatusDegraded:
		return "🟡"
	case StatusDown:
		return "🛑"
	default:
		return "⚪"
	}
}

// GetExternalIcon returns the external accessibility icon
func (s *ServerStatus) GetExternalIcon() string {
	if s.ExternalAccess {
		return "📶"
	}
	return "❌"
}

// FormatUptime returns a human-readable uptime string
func (s *ServerStatus) FormatUptime() string {
	if s.Uptime == 0 {
		return "unknown"
	}

	days := int(s.Uptime.Hours() / 24)
	hours := int(s.Uptime.Hours()) % 24
	minutes := int(s.Uptime.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// FormatProgressBar returns a text progress bar
func FormatProgressBar(percent float64, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	empty := width - filled

	return strings.Repeat("▓", filled) + strings.Repeat("░", empty)
}

// Ping checks if Prometheus is reachable
func (c *Checker) Ping() error {
	return c.prometheus.Ping()
}

// isCacheValid returns true if cache is still valid (within TTL)
func (c *Checker) isCacheValid() bool {
	if len(c.cache) == 0 {
		return false
	}
	return time.Since(c.cacheTime) < c.cacheTTL
}

// getCachedStatuses returns all cached statuses in order
func (c *Checker) getCachedStatuses() []*ServerStatus {
	var statuses []*ServerStatus
	for _, cloud := range c.config.Infrastructure.Clouds {
		for _, server := range cloud.Servers {
			if status, ok := c.cache[server.ID]; ok {
				statuses = append(statuses, status)
			}
		}
	}
	return statuses
}

// InvalidateCache clears the cache
func (c *Checker) InvalidateCache() {
	c.cache = make(map[string]*ServerStatus)
	c.cacheTime = time.Time{}
}
