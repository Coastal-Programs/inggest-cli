---
paths:
  - "worker/**"
---

# Cloudflare Worker Rules

## Language
- Plain JavaScript — no TypeScript, no build step, no bundler
- Uses native Workers runtime APIs only (no Node.js APIs)

## Endpoints

| Endpoint | Caller | Auth | Purpose |
|----------|--------|------|---------|
| `POST /init-session` | Zeus Electron app | `ZEUS_ADMIN_SECRET` header | Issues one-time session token (5 min TTL) |
| `POST /token` | Zeus CLI | Valid `session_token` in body | Exchanges PKCE code → Xero tokens + `instance_token` |
| `POST /refresh` | Zeus CLI | Valid `instance_token` in body | Refreshes Xero access token |

## KV Patterns
- Session tokens: key `session:{uuid}`, TTL 300s (5 min), deleted on first use
- Instance tokens: key `instance:{uuid}`, TTL 7,776,000s (90 days), `lastUsed` updated on each refresh
- One-time token consumption: `get` → if exists, `delete` → proceed; never re-check after delete

## Secrets
- `XERO_CLIENT_ID`, `XERO_CLIENT_SECRET`, `ZEUS_ADMIN_SECRET` — all set via `wrangler secret put`
- Never put secret values in `wrangler.toml` or source code

## Secret Comparison
- Always use `crypto.subtle.timingSafeEqual` for comparing secrets — never `===`
- Always compare equal-length buffers — if lengths differ, run a dummy comparison then return false (prevents timing attacks)

## Deploy
```bash
cd worker
npx wrangler deploy
```
