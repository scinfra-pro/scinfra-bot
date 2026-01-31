package prometheus

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Client provides access to Prometheus HTTP API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// QueryResult represents a single metric result
type QueryResult struct {
	Instance string
	Value    float64
}

// prometheusResponse represents the Prometheus API response
type prometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [timestamp, "value"]
		} `json:"result"`
	} `json:"data"`
	Error     string `json:"error,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
}

// NewClient creates a new Prometheus client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Query executes a PromQL query and returns results
func (c *Client) Query(promql string) ([]QueryResult, error) {
	endpoint := fmt.Sprintf("%s/api/v1/query", c.baseURL)

	params := url.Values{}
	params.Set("query", promql)

	resp, err := c.httpClient.Get(endpoint + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus returned status %d", resp.StatusCode)
	}

	var promResp prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&promResp); err != nil {
		return nil, fmt.Errorf("failed to decode prometheus response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("prometheus error: %s - %s", promResp.ErrorType, promResp.Error)
	}

	var results []QueryResult
	for _, r := range promResp.Data.Result {
		if len(r.Value) < 2 {
			continue
		}

		valueStr, ok := r.Value[1].(string)
		if !ok {
			continue
		}

		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		results = append(results, QueryResult{
			Instance: r.Metric["instance"],
			Value:    value,
		})
	}

	return results, nil
}

// QuerySingle executes a query and returns the first result value
func (c *Client) QuerySingle(promql string) (float64, error) {
	results, err := c.Query(promql)
	if err != nil {
		return 0, err
	}

	if len(results) == 0 {
		return 0, fmt.Errorf("no results for query: %s", promql)
	}

	return results[0].Value, nil
}

// IsUp checks if an instance is up (returns true if up == 1)
// instance can be either hostname (e.g., "edge-gateway") or IP:port (e.g., "10.0.1.11:9100")
func (c *Client) IsUp(instance string) (bool, error) {
	// First try exact match (for hostname-based labels like "edge-gateway")
	query := fmt.Sprintf(`up{instance="%s",job="node"}`, instance)
	value, err := c.QuerySingle(query)
	if err != nil {
		// Try with regex match
		query = fmt.Sprintf(`up{instance=~"%s.*",job="node"}`, instance)
		value, err = c.QuerySingle(query)
		if err != nil {
			return false, err
		}
	}
	return value == 1, nil
}

// GetCPU returns CPU usage percentage for an instance
func (c *Client) GetCPU(instance string) (float64, error) {
	query := fmt.Sprintf(
		`100 - avg(rate(node_cpu_seconds_total{mode="idle",instance="%s"}[5m]))*100`,
		instance,
	)
	return c.QuerySingle(query)
}

// GetMemory returns memory usage percentage for an instance
func (c *Client) GetMemory(instance string) (float64, error) {
	query := fmt.Sprintf(
		`(1 - node_memory_MemAvailable_bytes{instance="%s"}/node_memory_MemTotal_bytes{instance="%s"})*100`,
		instance, instance,
	)
	return c.QuerySingle(query)
}

// GetMemoryBytes returns memory usage in bytes (used, total)
func (c *Client) GetMemoryBytes(instance string) (used, total float64, err error) {
	totalQuery := fmt.Sprintf(`node_memory_MemTotal_bytes{instance="%s"}`, instance)
	total, err = c.QuerySingle(totalQuery)
	if err != nil {
		return 0, 0, err
	}

	availQuery := fmt.Sprintf(`node_memory_MemAvailable_bytes{instance="%s"}`, instance)
	avail, err := c.QuerySingle(availQuery)
	if err != nil {
		return 0, 0, err
	}

	used = total - avail
	return used, total, nil
}

// GetDisk returns disk usage percentage for an instance (root filesystem)
func (c *Client) GetDisk(instance string) (float64, error) {
	query := fmt.Sprintf(
		`(1 - node_filesystem_avail_bytes{instance="%s",mountpoint="/"}/node_filesystem_size_bytes{instance="%s",mountpoint="/"})*100`,
		instance, instance,
	)
	return c.QuerySingle(query)
}

// GetDiskBytes returns disk usage in bytes (used, total) for root filesystem
func (c *Client) GetDiskBytes(instance string) (used, total float64, err error) {
	totalQuery := fmt.Sprintf(`node_filesystem_size_bytes{instance="%s",mountpoint="/"}`, instance)
	total, err = c.QuerySingle(totalQuery)
	if err != nil {
		return 0, 0, err
	}

	availQuery := fmt.Sprintf(`node_filesystem_avail_bytes{instance="%s",mountpoint="/"}`, instance)
	avail, err := c.QuerySingle(availQuery)
	if err != nil {
		return 0, 0, err
	}

	used = total - avail
	return used, total, nil
}

// GetUptime returns the uptime of an instance
func (c *Client) GetUptime(instance string) (time.Duration, error) {
	query := fmt.Sprintf(
		`node_time_seconds{instance="%s"} - node_boot_time_seconds{instance="%s"}`,
		instance, instance,
	)
	seconds, err := c.QuerySingle(query)
	if err != nil {
		return 0, err
	}
	return time.Duration(seconds) * time.Second, nil
}

// IsServiceUp checks if a specific service/job is up
func (c *Client) IsServiceUp(job string, instance string) (bool, error) {
	var query string
	if instance != "" {
		query = fmt.Sprintf(`up{job="%s",instance=~"%s.*"}`, job, instance)
	} else {
		query = fmt.Sprintf(`up{job="%s"}`, job)
	}

	value, err := c.QuerySingle(query)
	if err != nil {
		return false, err
	}
	return value == 1, nil
}

// Ping checks if Prometheus is reachable
func (c *Client) Ping() error {
	endpoint := fmt.Sprintf("%s/-/healthy", c.baseURL)
	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return fmt.Errorf("prometheus not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus health check failed: status %d", resp.StatusCode)
	}

	return nil
}
