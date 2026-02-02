# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-02-02

### Added

- Asynchronous external IP fetching with 60-second in-memory cache
- VPS mode-aware IP caching (separate entries for direct/warp/home)
- Pending state indicator (`⏳checking...`) during IP fetch
- IPv4-only external IP detection via `api.ipify.org`

### Changed

- All mode switching operations now execute asynchronously
- Refresh button forces cache bypass and shows spinner during operation
- `/status` command uses cached IP when available

## [1.1.0] - 2026-02-01

### Added

- SSH monitoring for edge-gateway (latency, success/error counts)
- SSH monitoring for switch-gate upstreams (latency, success/error counts)
- SSH statistics section in infrastructure health details

### Changed

- VPS mode display now always shows real mode with health indicator
- Optimistic UI update after VPS mode switch (shows expected mode if status unavailable)

## [1.0.3] - 2026-01-31

### Fixed

- Panic when health checker is nil (S3 load failure graceful handling)
- Added nil checks to `buildHealthMessage` and `buildServerDetailMessage`

## [1.0.2] - 2026-01-31

### Added

- `prometheus_instance` field support in server metadata (schema v1.2)
- Health checker now uses `prometheus_instance` for Prometheus queries

### Changed

- Server `id` is now a unique machine-readable identifier from cloud provider
- Server `name` is now display name for UI (falls back to `server_name`)

## [1.0.1] - 2026-01-31

### Fixed

- S3 infrastructure clouds now replace YAML clouds instead of appending (was causing duplicates)

### Changed

- Updated `docs/configuration.md` with S3 dynamic configuration section

## [1.0.0] - 2026-01-31

### Added

- Dynamic infrastructure configuration from S3-compatible storage
- S3 metadata loader with configurable provider list
- Graceful fallback to YAML when S3 is unavailable
- Merge logic for edge, upstreams, and infrastructure from S3
- AWS SDK v2 integration for S3 operations
- `s3` configuration section with `enabled`, `bucket`, `endpoint`, `region`, `profile`, `providers` options
- Runtime validation after S3 merge

### Changed

- Configuration validation now supports S3-only mode (edge/upstreams from S3)
- Updated `configs/config.example.yaml` with S3 configuration examples

## [0.x.x] - Pre-release

Initial development versions with core functionality:

- Telegram bot with edge-gateway control
- VPN mode switching (direct, warp, home)
- Multi-upstream support with switch-gate integration
- Traffic monitoring from edge and upstreams
- Infrastructure monitoring via Prometheus
- Health checks (internal metrics + external probes)
- Webhook receiver for switch-gate notifications
