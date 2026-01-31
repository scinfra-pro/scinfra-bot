# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
