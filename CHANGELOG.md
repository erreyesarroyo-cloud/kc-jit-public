# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres
to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- Public install path is Docker Compose and generic Helm; AKS Terraform is optional lab-only.
- Helm chart defaults no longer point at a private lab registry or live IPs; `sessionSecret` must be set at deploy time.
- Grant-duration docs aligned to the shipped 9h default.

### Added
- Unit tests for store, session/header auth, and request → approve → release / reconcile.

## [0.1.0] - 2026-09-08

Initial public release.

### Added
- Just-in-time, time-bound admin elevation for Keycloak via the Admin REST API.
- Request / approve / early-release workflow with mandatory approver notes.
- Two-timer model: request TTL (default 1h) and active-grant TTL (default 9h).
- Expiry scheduler with startup reconciliation (no orphaned access after restart).
- Tamper-evident SQLite audit ledger for every request, approval, and revocation.
- Break-glass group with alerting on use.
- Self-service web UI with OIDC login.
- Optional approver notifications via webhook.
- Helm charts for Keycloak and the service (`deploy/helm/`).
- Docker Compose quickstart with a seeded demo realm.

[0.1.0]: https://github.com/erreyesarroyo-cloud/keycloak-jit-access/releases/tag/v0.1.0
