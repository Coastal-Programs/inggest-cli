# Zeus CLI — Architecture

## Overview

Zeus CLI is a command-line tool that provides structured access to the Xero Accounting API. It is the data layer for the Zeus Electron application — it fetches financial data from Xero and returns clean JSON that Zeus uses to power its AI chat interface.

It is **not a standalone public tool**. Every authentication attempt must be initiated through the Zeus application.

---

## System Components

```
┌─────────────────────────────────────────────────────────────┐
│                      Zeus App (Electron)                    │
│   - AI chat interface                                       │
│   - Initiates Xero authentication                          │
│   - Spawns CLI commands and parses their JSON output        │
└────────────────────────┬────────────────────────────────────┘
                         │ spawns process + sets env vars
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Zeus CLI  (xero binary)                  │
│   - All commands return structured JSON                     │
│   - Reads config from ~/.config/xero/config.json           │
│   - Never holds the Xero client_secret                      │
└────────────────────────┬────────────────────────────────────┘
                         │ token exchange + refresh
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Auth Proxy  (Cloudflare Worker)                │
│   - Only place the Xero client_secret lives                 │
│   - Validates session tokens issued by Zeus app             │
│   - Brokers all OAuth communication with Xero               │
│   - Stores session + instance tokens in Cloudflare KV      │
└────────────────────────┬────────────────────────────────────┘
                         │ authenticated API calls
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   Xero Accounting API                       │
│   - api.xero.com/api.xro/2.0                               │
│   - One access token, many organisations (multi-tenant)     │
└─────────────────────────────────────────────────────────────┘
```

---

## Authentication Flow

Zeus uses **OAuth 2.0 PKCE** with a proxy layer so the `client_secret` never touches a user's machine.

### First-time login

```
Zeus App                  CLI                  Auth Proxy              Xero
   │                       │                       │                     │
   │─── POST /init-session ──────────────────────► │                     │
   │    (Authorization: Bearer ZEUS_ADMIN_SECRET)  │                     │
   │                       │                       │                     │
   │◄── { session_token } ─────────────────────── │                     │
   │    (one-time, 5 min TTL, stored in KV)        │                     │
   │                       │                       │                     │
   │── spawn CLI with ────►│                       │                     │
   │   ZEUS_SESSION_TOKEN  │                       │                     │
   │   env var             │                       │                     │
   │                       │── open browser ──────────────────────────► │
   │                       │   (PKCE challenge)    │                     │
   │                       │                       │                     │
   │                       │◄─ auth code via ─────────────────────────── │
   │                       │   localhost:8765      │                     │
   │                       │                       │                     │
   │                       │── POST /token ───────►│                     │
   │                       │   session_token       │                     │
   │                       │   code                │── POST /token ────► │
   │                       │   code_verifier       │   + client_secret   │
   │                       │                       │                     │
   │                       │                       │◄─ access_token ──── │
   │                       │                       │   refresh_token     │
   │                       │                       │                     │
   │                       │◄─ access_token ───── │                     │
   │                       │   refresh_token       │                     │
   │                       │   instance_token      │  (KV: instance      │
   │                       │                       │   token stored,     │
   │                       │                       │   90 day TTL)       │
   │                       │                       │                     │
   │                       │── save to ────────────                      │
   │                       │   config.json         │                     │
```

### Token refresh (automatic, background)

```
CLI                          Auth Proxy              Xero
 │                               │                     │
 │── POST /refresh ─────────────►│                     │
 │   instance_token              │                     │
 │   refresh_token               │── POST /token ────► │
 │                               │   + client_secret   │
 │                               │                     │
 │◄─ new access_token ──────────│◄─ new access_token ─│
```

### What blocks unauthorised access

| Attempt | Result |
|---|---|
| Run `xero auth login` directly in terminal | Error: must be started from Zeus app |
| Hit `/init-session` without `ZEUS_ADMIN_SECRET` | 401 Unauthorized |
| Hit `/token` without a valid session token | 401 Unauthorized |
| Hit `/token` with a session token twice | 401 — token consumed on first use |
| Hit `/refresh` without a valid instance token | 401 Unauthorized |
| Session token not used within 5 minutes | 401 — expired in KV |

---

## Project Structure

```
zeus-cli/
│
├── cmd/xero/
│   └── main.go                  # Entry point. Injects version, clientID, proxyURL
│                                # via ldflags at build time.
│
├── internal/
│   ├── auth/
│   │   ├── oauth.go             # OAuth 2.0 PKCE flow, proxy-aware token exchange
│   │   └── assets/
│   │       └── logo.png         # Zeus logo — embedded into binary for callback page
│   │
│   ├── cli/
│   │   ├── root.go              # Root Cobra command, global --org and --output flags
│   │   └── commands/
│   │       ├── auth.go          # xero auth login/logout/status/refresh
│   │       ├── orgs.go          # xero orgs list/use/sync
│   │       ├── invoices.go      # xero invoices list/get/create/void/email
│   │       ├── contacts.go      # xero contacts list/get/create/update
│   │       ├── accounts.go      # xero accounts list/get
│   │       ├── payments.go      # xero payments list/get/create
│   │       ├── reports.go       # xero reports profit-loss/balance-sheet/etc
│   │       ├── bank.go          # xero bank accounts/transactions/get
│   │       ├── items.go         # xero items list/get/create
│   │       ├── config.go        # xero config get/set/show
│   │       └── version.go       # xero version
│   │
│   ├── xero/
│   │   ├── client.go            # HTTP client with auth, User-Agent, 429 handling
│   │   ├── invoices.go          # Invoice API methods + auto-pagination
│   │   ├── contacts.go          # Contact API methods + auto-pagination
│   │   ├── accounts.go          # Chart of accounts API methods
│   │   ├── payments.go          # Payment API methods
│   │   ├── reports.go           # Financial report API methods
│   │   ├── bank.go              # Bank account/transaction API methods
│   │   └── items.go             # Inventory item API methods
│   │
│   └── common/
│       └── config/
│           └── config.go        # Config load/save, tenant resolution, secret redaction
│
├── pkg/
│   └── output/
│       └── output.go            # JSON / text / table output formatter
│
├── worker/
│   ├── src/
│   │   └── index.js             # Cloudflare Worker — auth proxy
│   ├── wrangler.toml            # Worker config + KV namespace binding
│   └── package.json
│
├── scripts/
│   └── release.sh               # Cross-platform release build (5 targets)
│
├── .github/
│   ├── workflows/
│   │   ├── test.yml             # CI: build + vet + test on push/PR
│   │   └── release.yml          # CD: build + publish GitHub Release on v* tag
│   ├── release.yml              # Auto release notes categories (PR labels)
│   └── ISSUE_TEMPLATE/
│       ├── bug_report.md
│       └── feature_request.md
│
├── CLAUDE.md                    # AI assistant project memory and conventions
├── ARCHITECTURE.md              # This file
├── README.md                    # Public-facing documentation
├── CHANGELOG.md                 # Version history
├── Makefile                     # Build, test, release, worker targets
├── go.mod                       # Module: github.com/jakeschepis/zeus-cli
└── .gitignore
```

---

## Technology Stack

| Layer | Technology | Why |
|---|---|---|
| CLI language | Go 1.23 | Fast binary, easy cross-compilation, strong stdlib |
| CLI framework | Cobra | Standard Go CLI framework, subcommand support |
| Auth proxy | Cloudflare Workers | Edge compute, free tier, no server to manage |
| Token storage | Cloudflare KV | Serverless key-value with TTL, built into Workers |
| Xero auth | OAuth 2.0 + PKCE | Industry standard, no client_secret on device |
| Output format | JSON (default) | Machine-readable for AI agent consumption |
| Local config | `~/.config/xero/config.json` | 0600 permissions, standard XDG location |
| Build injection | Go ldflags | Embeds clientID + proxyURL at compile time |
| Releases | GitHub Actions + shell script | Cross-compiles 5 targets, auto GitHub Release |

---

## Multi-Organisation Support

Each retirement agency is a separate Xero organisation. Zeus CLI connects all of them in a single login and lets you target any one — or all at once.

```
One access token
       │
       ├── Organisation A (Xero-Tenant-Id: aaa...)  ← active (default)
       ├── Organisation B (Xero-Tenant-Id: bbb...)
       └── Organisation C (Xero-Tenant-Id: ccc...)
```

Every API request sets the `Xero-Tenant-Id` header to target the right org.

**Relevant flags on most commands:**

| Flag | Behaviour |
|---|---|
| _(no flag)_ | Uses the active org |
| `--org "Agency Name"` | Targets org by name (partial match) or ID |
| `--all-orgs` | Runs the command across every connected org, returns combined JSON |

---

## Local Config File

Stored at `~/.config/xero/config.json` with `0600` permissions (owner read/write only).

```json
{
  "client_id":        "...",
  "access_token":     "...",
  "refresh_token":    "...",
  "token_expiry":     1234567890,
  "instance_token":   "...",
  "active_tenant_id": "...",
  "tenants": [
    { "tenant_id": "aaa...", "tenant_name": "Agency A" },
    { "tenant_id": "bbb...", "tenant_name": "Agency B" }
  ]
}
```

`client_secret` is **never stored locally** — it exists only in the Cloudflare Worker.

---

## Build & Release

### Development build
```bash
export CLIENT_ID=your-xero-client-id
export PROXY_URL=https://zeus-auth-proxy.curly-cherry-d5dc.workers.dev
make build          # outputs ./build/xero
```

### Cutting a release
```bash
git tag -a v0.2.0 -m "Release v0.2.0"
git push origin v0.2.0
# GitHub Actions builds 5 platform binaries and publishes the GitHub Release
```

The `CLIENT_ID` and `PROXY_URL` are stored as GitHub Actions secrets (`XERO_CLIENT_ID`, `PROXY_URL`) and injected at build time — they are never in source code.

### Release targets
| OS | Architecture |
|---|---|
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Linux | amd64, arm64 |
| Windows | amd64 |

---

## Deployed Endpoints

| Resource | Value |
|---|---|
| Worker URL | `https://zeus-auth-proxy.curly-cherry-d5dc.workers.dev` |
| KV Namespace ID | `25d1d6f714964884ab762ba16135ee57` |
| Cloudflare Account | `info@coastalprograms.com` |

---

## Cloudflare Worker — Secrets Reference

| Secret | Set via | Purpose |
|---|---|---|
| `XERO_CLIENT_ID` | `wrangler secret put` | Xero OAuth app client ID |
| `XERO_CLIENT_SECRET` | `wrangler secret put` | Xero OAuth app client secret |
| `ZEUS_ADMIN_SECRET` | `wrangler secret put` | Protects `/init-session` — only Zeus app holds this |

| KV Namespace | Purpose |
|---|---|
| `SESSIONS` | Stores session tokens (5 min TTL) and instance tokens (90 day TTL) |
