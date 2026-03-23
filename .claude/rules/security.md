# Security Rules

## Keys and Secrets
- Never log or print raw signing keys or event keys — use redaction helpers
- Signing key (`INNGEST_SIGNING_KEY`) and event key (`INNGEST_EVENT_KEY`) can come from env vars or config file
- Env vars take precedence over config file values

## Config File
- Always written with `0600` permissions — do not change this
- `INNGEST_CLI_CONFIG` env var overrides the default path

## HTTP
- All HTTP clients must have explicit timeouts — no bare `http.DefaultClient`
- Error response bodies are truncated before logging — never log full upstream error bodies
