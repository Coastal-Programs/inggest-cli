# Security Rules

## Tokens and Secrets
- Never log or print raw tokens — use `config.Redacted()` or the `redact()` helper
- `client_secret` must NEVER appear in the CLI binary, config file, or logs
- It lives exclusively in the Cloudflare Worker as a Cloudflare secret

## Config File
- Always written with `0600` permissions — do not change this
- `XERO_CONFIG` env var is validated: must be absolute path, end in `.json`, no `..` — do not relax this validation

## Auth Proxy
- `/init-session` is protected by `ZEUS_ADMIN_SECRET` — only the Zeus Electron app holds this
- `instance_token` must be validated on every `/refresh` call — proves the CLI was authorised through Zeus
- Session tokens are single-use — consumed (deleted from KV) on first use
- Admin secret comparison uses `crypto.subtle.timingSafeEqual` — never use `===` for secret comparison

## HTTP
- All HTTP clients must have explicit timeouts — no bare `http.DefaultClient`
- Error response bodies are truncated to 300 chars before logging — never log full upstream error bodies
