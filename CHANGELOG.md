# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-02-20

### Added
- OAuth 2.0 PKCE authentication flow with browser-based login
- Multi-org support: connect multiple Xero organisations, switch with `--org` flag
- `--all-orgs` flag on key commands to aggregate results across all connected orgs
- `xero orgs` command group: `list`, `use`, `sync`
- `xero invoices` — list (auto-paginated), get, create, void, email
- `xero contacts` — list (auto-paginated), get, create, update
- `xero accounts` — list, get
- `xero payments` — list, get, create
- `xero reports` — profit-loss, balance-sheet, trial-balance, aged-receivables, aged-payables
- `xero bank` — accounts, transactions (auto-paginated), get
- `xero items` — list, get, create
- `xero config` — get, set, show (with secret redaction)
- `xero version` command
- Automatic 429 rate limit handling with `Retry-After` respect
- JSON, text, and table output formats via `--output` flag
- Styled OAuth callback page
