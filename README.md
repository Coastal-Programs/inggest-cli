# xero CLI

A fast, scriptable command-line interface for the [Xero Accounting API](https://developer.xero.com/documentation/api/accounting/overview). Returns structured JSON output — ideal for AI agents, shell scripts, and data pipelines.

## Features

- **OAuth 2.0 PKCE** authentication — no manual token handling
- **Multi-org support** — query across multiple Xero organisations with `--org` or `--all-orgs`
- **Auto-pagination** — fetches all records automatically
- **JSON by default** — every command outputs clean JSON; use `--output table` or `--output text` for human-readable output
- **Rate limit aware** — automatically respects Xero's `Retry-After` headers on 429 responses

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

## Prerequisites

1. Create a **Web App** at [developer.xero.com](https://developer.xero.com/myapps)
2. Set the OAuth 2.0 redirect URI to: `http://localhost:8765/callback`
3. Note your **Client ID** and **Client Secret**

## Quick Start

```bash
# 1. Set credentials
xero config set client_id     YOUR_CLIENT_ID
xero config set client_secret YOUR_CLIENT_SECRET

# 2. Authenticate (opens browser)
xero auth login

# 3. Start querying
xero invoices list --status AUTHORISED
xero reports profit-loss --from 2024-01-01 --to 2024-12-31
```

## Commands

```
xero version                          Print CLI version
xero auth login                       Authenticate via browser (PKCE)
xero auth logout                      Clear stored tokens
xero auth status                      Show authentication status
xero auth refresh                     Manually refresh access token

xero orgs list                        List all connected organisations
xero orgs use <name-or-id>            Set active organisation
xero orgs sync                        Re-fetch orgs from Xero

xero invoices list                    List invoices
xero invoices get <id>                Get invoice by ID
xero invoices create                  Create invoice
xero invoices void <id>               Void invoice
xero invoices email <id>              Email invoice to contact

xero contacts list                    List contacts
xero contacts get <id>                Get contact by ID
xero contacts create                  Create contact
xero contacts update <id>             Update contact

xero accounts list                    List chart of accounts
xero accounts get <id-or-code>        Get account

xero payments list                    List payments
xero payments get <id>                Get payment
xero payments create                  Apply payment to invoice

xero reports profit-loss              Profit & Loss report
xero reports balance-sheet            Balance Sheet report
xero reports trial-balance            Trial Balance report
xero reports aged-receivables         Aged Receivables report
xero reports aged-payables            Aged Payables report

xero bank accounts                    List bank accounts
xero bank transactions                List bank transactions
xero bank get <id>                    Get bank transaction

xero items list                       List inventory items
xero items get <id-or-code>           Get item
xero items create                     Create item

xero config get <key>                 Get config value
xero config set <key> <value>         Set config value
xero config show                      Show all config (secrets redacted)
```

## Multi-Org Usage

If you manage multiple Xero organisations (e.g. multiple agencies), connect them all in one login and query any of them:

```bash
# List all connected orgs
xero orgs list

# Target a specific org
xero invoices list --org "Agency Name"
xero reports profit-loss --org "Agency B" --from 2024-01-01 --to 2024-12-31

# Run across all orgs at once
xero reports balance-sheet --all-orgs
xero invoices list --all-orgs --status AUTHORISED
```

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--org` | active org | Target org by name or ID |
| `--output`, `-o` | `json` | Output format: `json`, `text`, `table` |

## Configuration

Config is stored at `~/.config/xero/config.json` with `0600` permissions.
Override location with the `XERO_CONFIG` environment variable (must be an absolute path).

```bash
xero config show      # view all settings (secrets redacted)
```

## Rate Limits

Xero enforces 60 calls/minute per org and 5,000 calls/day. The CLI automatically handles `429 Too Many Requests` responses by reading the `Retry-After` header and waiting before retrying.

## License

MIT — see [LICENSE](LICENSE)
