# Zeus CLI

Go CLI for the Xero Accounting API. Data layer for the Zeus Electron app.
Serves Australian retirement agencies — each agency is a separate Xero org.

- **Module:** `github.com/jakeschepis/zeus-cli`
- **Binary:** `xero`
- **Config:** `~/.config/xero/config.json`
- **Only external dep:** `github.com/spf13/cobra v1.8.1` — prefer stdlib for everything else

## Commands

```bash
make build      # → ./build/xero
make install    # → $GOPATH/bin
make check      # fmt + vet + test — run before every commit
make clean      # remove build/dist artifacts
```

```bash
./build/xero auth status
./build/xero orgs list
./build/xero invoices list --status AUTHORISED
```

## Structure

```
cmd/xero/main.go                  # entry point, ldflags: version, clientID, proxyURL
internal/cli/root.go              # root Cobra command, --org and --output flags
internal/cli/commands/            # one file per command group
internal/xero/                    # Xero API client + resource methods
internal/auth/oauth.go            # OAuth 2.0 PKCE + proxy-aware token exchange
internal/auth/assets/logo.png     # embedded Zeus logo (callback page + favicon)
internal/common/config/config.go  # config load/save, tenant resolution
pkg/output/output.go              # JSON / text / table formatter
worker/src/index.js               # Cloudflare Worker — auth proxy
scripts/release.sh                # cross-platform release builder
```

## Detailed Rules

@.claude/rules/go.md
@.claude/rules/security.md
@.claude/rules/release.md
@.claude/rules/worker.md
