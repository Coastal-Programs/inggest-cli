# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-02-24

This release closes the full Xero Accounting API gap, adding credit notes, tracking categories, manual journals, purchase orders, budgets, overpayments, prepayments, quotes, tax rates, and five new financial reports.

### Added
- `xero credit-notes` — list (auto-paginated), get, create, apply against invoice (`--credit-note-id`, `--invoice-id`, `--amount`)
- `xero tracking` — list, get, create tracking categories; `option add` to add category options
- `xero journals` — list (auto-paginated), get, create manual journals
- `xero journal-ledger` — list journal ledger entries with offset-based pagination (read-only audit trail)
- `xero po` — list (auto-paginated), get, create purchase orders
- `xero budgets` — list and get budgets (read-only; created in Xero UI)
- `xero overpayments` — list (auto-paginated), apply against invoice
- `xero prepayments` — list (auto-paginated), apply against invoice
- `xero quotes` — list (auto-paginated), get, create, convert to invoice
- `xero tax-rates list` — list tax rates (reference data)
- `xero reports cash-flow` — Cash Flow Summary report
- `xero reports budget-variance` — Budget Variance report (`--budget-id` required)
- `xero reports account-transactions` — Account Transactions report (`--account-code` required)
- `xero reports executive-summary` — Executive Summary report
- `xero reports bank-summary` — Bank Summary report
- `--tracking-category` and `--tracking-option` flags on `xero reports profit-loss` for segment filtering

## [0.2.0] - 2026-02-20

This release introduces the Cloudflare Worker auth proxy, which removes the Xero `client_secret` from the CLI binary entirely. Authentication is now gated through the Zeus Electron app — users cannot run `xero auth login` directly without being authorised through Zeus first.

### Added
- Cloudflare Worker auth proxy (`worker/`) — `client_secret` now lives exclusively in Cloudflare, never in the CLI binary
- Session token gating: `xero auth login` can only be initiated through the Zeus Electron app (requires `ZEUS_SESSION_TOKEN` env var set by Zeus)
- `POST /init-session` worker endpoint — Zeus app calls this with `ZEUS_ADMIN_SECRET` to issue a one-time 5-minute session token
- `POST /token` worker endpoint — CLI exchanges PKCE auth code via proxy; consumes session token and returns Xero tokens + long-lived `instance_token`
- `POST /refresh` worker endpoint — CLI refreshes Xero access token via proxy; validates `instance_token` to prove CLI was authorised through Zeus
- Cloudflare KV (`SESSIONS` namespace) for session and instance token storage with TTL
- `instance_token` field in local config (`~/.config/xero/config.json`)
- `defaultClientID` and `proxyURL` build-time vars injected via ldflags — users no longer need to run `xero config set client_id`
- GitHub Actions secrets `XERO_CLIENT_ID` and `PROXY_URL` wired into release workflow
- `.claude/rules/` project memory restructure: `go.md`, `security.md`, `release.md`, `worker.md`
- `ARCHITECTURE.md` system design document

### Fixed
- Errors from all commands now printed as JSON to stderr — previously silently swallowed due to `SilenceErrors: true` on root command

### Security
- `client_secret` removed from CLI binary entirely — only exists as an encrypted Cloudflare secret
- Worker validates `redirect_uri` on every token exchange to prevent auth code injection
- Admin secret comparison uses `crypto.subtle.timingSafeEqual` (timing-safe, per Cloudflare docs)
- Session tokens are single-use — consumed and deleted from KV on first use

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
