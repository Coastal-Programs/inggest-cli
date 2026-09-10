# Security Policy

## Supported Versions

| Version | Supported          | Notes                          |
| ------- | ------------------ | ------------------------------ |
| 0.3.x   | :white_check_mark: | Current release, REST API v2   |
| 0.2.x   | :x:                | Used the undocumented dashboard GraphQL API; no longer maintained |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Report privately through either channel:

- **GitHub Security Advisories** — use the **Report a vulnerability** button on
  the [Security tab](https://github.com/Coastal-Programs/inggest-cli/security).
  This keeps the report private until a fix ships.
- **Email** — `jake@coastalprograms.com`

Please include:

- The type of vulnerability and its impact
- Affected source files and the tag, branch or commit
- Step-by-step reproduction instructions
- A proof of concept, if you have one
- A suggested fix, if you have one

### What to expect

| Stage             | Timeline                            |
| ----------------- | ----------------------------------- |
| Acknowledgement   | Within 48 hours                     |
| Status update     | Every 7 days until resolved         |
| Fix target        | Within 90 days; critical issues are expedited |

We follow **coordinated disclosure**: please allow time for a fix before public
disclosure. You will be credited in the release notes unless you prefer
anonymity.

## Credential Handling

The CLI authenticates to Inngest with an API key, a signing key, or an event
key. How each is stored and displayed:

### Storage

- **Config file**: `cli.json`, resolved in this order —
  1. `INNGEST_CLI_CONFIG` (explicit override)
  2. `$XDG_CONFIG_HOME/inngest/cli.json`
  3. `os.UserConfigDir()/inngest/cli.json` — `~/Library/Application Support` on
     macOS, `%AppData%` on Windows, `~/.config` on Linux
  4. `~/.config/inngest/cli.json` as a fallback
- **File permissions**: `0o600` (owner read/write only)
- **Directory permissions**: `0o700` (owner access only)

> **Note:** the config file is written with a plain `os.WriteFile`, not an
> atomic temp-file-and-rename. An interrupted write can truncate the file. Back
> up the file if you keep anything you cannot re-enter, and re-run
> `inngest auth login` if it is corrupted.

### Environment variables

These take precedence over the config file, in this order:

1. `INNGEST_API_KEY`
2. `INNGEST_SIGNING_KEY` (with `INNGEST_SIGNING_KEY_FALLBACK` for key rotation)
3. Config file `api_key`
4. Config file `signing_key`

`INNGEST_EVENT_KEY` is used separately, for sending events.

### Display

Credentials are redacted in all command output that echoes them back —
`auth status`, `auth login` and the `config` commands. Redaction keeps the
first four and last four characters (`sign****f3a2`); anything eight characters
or shorter is replaced entirely with `****`.

There is **no** flag to print a stored credential in full. Read the config file
directly if you need the raw value.

## Transport

- All API traffic goes to the configured base URL over HTTPS. Certificate
  verification is never disabled.
- Credentials are sent in the `Authorization` header only — never in a URL
  path, query string, or log line.
- The `inngest api` passthrough command accepts only relative paths. Absolute
  URLs (`https://…`) and protocol-relative paths (`//host`) are rejected, so a
  crafted path cannot redirect your credential to another host.
- API responses are read with a 25 MB cap to bound memory use.

## Supply Chain

- **2 direct Go dependencies**: `github.com/spf13/cobra` and
  `golang.org/x/term`. `go.mod` lists 5 modules including indirect ones; a
  build links 4 of them (`mousetrap` is Windows-only). Verify for yourself
  with `go version -m $(which inngest)`.
- **Single static binary** — no interpreter or runtime needed at execution.
- **No npm production dependencies**: the npm package is a thin wrapper that
  runs the platform-specific Go binary.
- **No code execution**: the CLI does not evaluate user-supplied code, load
  plugins, or shell out to external programs.
- **Release binaries** are cross-compiled by GitHub Actions from a tagged
  commit for darwin/amd64, darwin/arm64, linux/amd64, linux/arm64 and
  windows/amd64. Each release publishes a `checksums.txt`.

### Verifying a download

```bash
# From the release assets
shasum -a 256 -c checksums.txt

# Confirm the binary matches the release you expect
inngest version
```

## Guidance for Users

**Never commit a signing key, API key or event key to version control.**

```bash
# Good: store it in the config file with 0600 permissions
inngest auth login

# Good: environment variable, scoped to the session or your shell profile
export INNGEST_API_KEY="..."

# Bad: hardcoding it in a committed file
echo "INNGEST_API_KEY=..." >> .env && git add .env
```

Additional recommendations:

1. Prefer an **API key** over a signing key for CI and automation.
2. **Rotate keys regularly.** `INNGEST_SIGNING_KEY_FALLBACK` exists so you can
   rotate without downtime.
3. **Add `.env` to `.gitignore`** if you use env files.
4. **Use a separate key per environment**; do not reuse a production key
   locally.
5. In CI, store keys as **encrypted secrets**, never as plaintext workflow
   values.
