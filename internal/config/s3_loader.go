package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config configures S3 metadata loading
// When Enabled=true: load from S3, fallback to YAML if unavailable
// When Enabled=false: use YAML only, skip S3 completely
type S3Config struct {
	Enabled   bool     `yaml:"enabled"`
	Bucket    string   `yaml:"bucket"`
	Prefix    string   `yaml:"prefix"`    // e.g. "metadata/"
	Endpoint  string   `yaml:"endpoint"`  // S3-compatible endpoint URL
	Region    string   `yaml:"region"`    // S3 region
	Profile   string   `yaml:"profile"`   // AWS CLI profile name
	Providers []string `yaml:"providers"` // List of provider JSON files to load
}

// S3Metadata represents combined metadata from all providers
type S3Metadata struct {
	Edge       *EdgeConfig
	Upstreams  map[string]*Upstream
	Clouds     []CloudConfig
}

// ProviderMetadata represents metadata from a single provider JSON file
type ProviderMetadata struct {
	SchemaVersion string `json:"schema_version"`
	Provider      string `json:"provider"` // provider identifier

	// Cloud info
	Cloud struct {
		Name string `json:"name"`
		Icon string `json:"icon"`
	} `json:"cloud"`

	// Servers
	Servers []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Icon          string `json:"icon"`
		IP            string `json:"ip"`
		ExternalIP    string `json:"external_ip"`
		ExternalCheck string `json:"external_check"`
		Services      []struct {
			Name string `json:"name"`
			Job  string `json:"job"`
			Port int    `json:"port"`
		} `json:"services"`
	} `json:"servers"`

	// Edge config (optional, for main cloud provider)
	Edge *struct {
		Name          string `json:"name"`
		Host          string `json:"host"`
		VPNModeScript string `json:"vpn_mode_script"`
	} `json:"edge,omitempty"`

	// Upstream config (optional, for VPS providers)
	Upstream *struct {
		Key            string `json:"key"`
		Name           string `json:"name"`
		IP             string `json:"ip"`
		User           string `json:"user"`
		SwitchGate     bool   `json:"switch_gate"`
		SwitchGatePort int    `json:"switch_gate_port"`
	} `json:"upstream,omitempty"`
}

// S3Loader loads metadata from S3
type S3Loader struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Loader creates a new S3 metadata loader
func NewS3Loader(cfg S3Config) (*S3Loader, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	// Load AWS config
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	// Use profile if specified
	if cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	// Create S3 client with custom endpoint
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	})

	prefix := cfg.Prefix
	if prefix == "" {
		prefix = "metadata/"
	}

	return &S3Loader{
		client: client,
		bucket: cfg.Bucket,
		prefix: prefix,
	}, nil
}

// Load fetches all metadata files from S3 and combines them
func (l *S3Loader) Load(ctx context.Context, providers []string) (*S3Metadata, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	metadata := &S3Metadata{
		Upstreams: make(map[string]*Upstream),
		Clouds:    []CloudConfig{},
	}

	for _, file := range providers {
		key := l.prefix + file
		data, err := l.fetchObject(ctx, key)
		if err != nil {
			log.Printf("Warning: failed to load %s: %v", key, err)
			continue
		}

		var pm ProviderMetadata
		if err := json.Unmarshal(data, &pm); err != nil {
			log.Printf("Warning: failed to parse %s: %v", key, err)
			continue
		}

		// Process metadata
		l.processMetadata(&pm, metadata)
		log.Printf("Loaded metadata from s3://%s/%s (provider: %s)", l.bucket, key, pm.Provider)
	}

	return metadata, nil
}

// fetchObject downloads an object from S3
func (l *S3Loader) fetchObject(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := l.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(l.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = result.Body.Close() }()

	return io.ReadAll(result.Body)
}

// processMetadata converts provider metadata to config structures
func (l *S3Loader) processMetadata(pm *ProviderMetadata, metadata *S3Metadata) {
	// Edge config (from main cloud provider)
	if pm.Edge != nil {
		metadata.Edge = &EdgeConfig{
			Name:          pm.Edge.Name,
			Host:          pm.Edge.Host,
			VPNModeScript: pm.Edge.VPNModeScript,
		}
	}

	// Upstream config (from VPS providers)
	if pm.Upstream != nil {
		metadata.Upstreams[pm.Upstream.Key] = &Upstream{
			Name:           pm.Upstream.Name,
			IP:             pm.Upstream.IP,
			User:           pm.Upstream.User,
			SwitchGate:     pm.Upstream.SwitchGate,
			SwitchGatePort: pm.Upstream.SwitchGatePort,
		}
	}

	// Cloud with servers
	if pm.Cloud.Name != "" && len(pm.Servers) > 0 {
		cloud := CloudConfig{
			Name:    pm.Cloud.Name,
			Icon:    pm.Cloud.Icon,
			Servers: make([]ServerConfig, 0, len(pm.Servers)),
		}

		for _, s := range pm.Servers {
			services := make([]ServiceConfig, 0, len(s.Services))
			for _, svc := range s.Services {
				services = append(services, ServiceConfig{
					Name: svc.Name,
					Job:  svc.Job,
					Port: svc.Port,
				})
			}

			cloud.Servers = append(cloud.Servers, ServerConfig{
				ID:            s.ID,
				Name:          s.Name,
				Icon:          s.Icon,
				IP:            s.IP,
				ExternalCheck: s.ExternalCheck,
				Services:      services,
			})
		}

		metadata.Clouds = append(metadata.Clouds, cloud)
	}
}
