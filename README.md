<div align="center">
  <img src="internal/auth/assets/logo.png" alt="Zeus CLI" width="110" />
  <h1>Zeus CLI</h1>
  <p><strong>A fast, scriptable Xero Accounting CLI built for AI agents and data pipelines.</strong></p>

  [![GitHub Release](https://img.shields.io/github/v/release/jakeschepis/zeus-cli?style=flat-square&color=gold)](https://github.com/jakeschepis/zeus-cli/releases)
  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)
  [![Go](https://img.shields.io/github/go-mod/go-version/jakeschepis/zeus-cli?style=flat-square)](go.mod)
</div>

---

Zeus CLI is a command-line tool for the [Xero Accounting API](https://developer.xero.com/documentation/api/accounting/overview). Every command returns structured JSON — purpose-built for AI assistants, shell scripts, and financial data pipelines.

## Features

- **OAuth 2.0 PKCE** — browser-based login with no manual token handling
- **Multi-org support** — manage multiple Xero organisations; switch with `--org` or query all with `--all-orgs`
- **Auto-pagination** — fetches every record automatically, no page loops needed
- **JSON by default** — clean, structured output; use `--output table` or `--output text` for humans
- **Rate limit aware** — reads Xero's `Retry-After` header on 429s and retries automatically

## Prerequisites

1. Create a **Web App** at [developer.xero.com/myapps](https://developer.xero.com/myapps)
2. Set the OAuth 2.0 redirect URI to `http://localhost:8765/callback`
3. Copy your **Client ID** and **Client Secret**

## Installation

**Requires Go 1.23+**

```bash
go install github.com/jakeschepis/zeus-cli/cmd/xero@latest
```

Or build from source:

```bash
git clone https://github.com/jakeschepis/zeus-cli
cd zeus-cli
make install
```

## Quick Start

```bash
# 1. Store your Xero app credentials
xero config set client_id     YOUR_CLIENT_ID
xero config set client_secret YOUR_CLIENT_SECRET

# 2. Authenticate (opens browser)
xero auth login

# 3. Start querying
xero invoices list --status AUTHORISED
xero reports profit-loss --from 2024-01-01 --to 2024-12-31
```

## Commands

### Auth

```bash
xero auth login          # Authenticate via browser (PKCE flow)
xero auth logout         # Clear stored tokens
xero auth status         # Show authentication status
xero auth refresh        # Manually refresh access token
```

### Organisations

```bash
xero orgs list                  # List all connected organisations
xero orgs use <name-or-id>      # Set the active organisation
xero orgs sync                  # Re-fetch organisations from Xero
```

### Invoices

```bash
xero invoices list              # List invoices
xero invoices get <id>          # Get invoice by ID
xero invoices create            # Create an invoice
xero invoices void <id>         # Void an invoice
xero invoices email <id>        # Email invoice to contact
```

Flags: `--status`, `--type`, `--from`, `--to`, `--page`, `--org`, `--all-orgs`

### Contacts

```bash
xero contacts list              # List contacts
xero contacts get <id>          # Get contact by ID
xero contacts create            # Create a contact
xero contacts update <id>       # Update a contact
```

### Accounts

```bash
xero accounts list              # List chart of accounts
xero accounts get <id-or-code>  # Get an account
```

### Payments

```bash
xero payments list              # List payments
xero payments get <id>          # Get a payment
xero payments create            # Apply a payment to an invoice
```

### Reports

```bash
xero reports profit-loss        # Profit & Loss
xero reports balance-sheet      # Balance Sheet
xero reports trial-balance      # Trial Balance
xero reports aged-receivables   # Aged Receivables
xero reports aged-payables      # Aged Payables
```

Flags: `--from`, `--to`, `--org`, `--all-orgs`

### Bank

```bash
xero bank accounts              # List bank accounts
xero bank transactions          # List bank transactions
xero bank get <id>              # Get a bank transaction
```

### Items

```bash
xero items list                 # List inventory items
xero items get <id-or-code>     # Get an item
xero items create               # Create an item
```

### Config

```bash
xero config get <key>           # Get a config value
xero config set <key> <value>   # Set a config value
xero config show                # Show all config (secrets redacted)
```

## Multi-Org Usage

Zeus CLI is designed for managing multiple Xero organisations — connect them all in a single login and query any of them on demand:

```bash
# See all connected orgs
xero orgs list

# Target a specific org by name (partial match)
xero invoices list --org "Retirement Agency A"
xero reports profit-loss --org "Agency B" --from 2024-01-01 --to 2024-12-31

# Run the same command across every org at once
xero reports balance-sheet --all-orgs
xero invoices list --all-orgs --status AUTHORISED
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | active org | Target org by name or ID |
| `--output`, `-o` | `json` | Output format: `json`, `text`, `table` |

## Configuration

Config is stored at `~/.config/xero/config.json` with `0600` permissions. Override the path with the `XERO_CONFIG` environment variable (must be an absolute path ending in `.json`).

```bash
xero config show    # View all settings (secrets redacted)
```

## Rate Limits

Xero enforces **60 calls/minute per org** and **5,000 calls/day**. Zeus CLI reads the `Retry-After` header on `429 Too Many Requests` responses and retries automatically — no manual intervention needed.

## License

[MIT](LICENSE) © 2026 Jake Schepis
