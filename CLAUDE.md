# Inngest CLI

Go CLI for the Inngest platform. Monitor, debug, and manage Inngest functions from the terminal.
Built for AI agents, shell scripts, and CI/CD pipelines.

- **Module:** `github.com/Coastal-Programs/inggest-cli`
- **Binary:** `inngest`
- **Config:** `~/.config/inngest/cli.json`
- **Only external dep:** `github.com/spf13/cobra v1.8.1` — prefer stdlib for everything else

## Commands

```bash
make build      # → ./build/inngest
make install    # → $GOPATH/bin
make check      # fmt + vet + test — run before every commit
make clean      # remove build/dist artifacts
```

```bash
./build/inngest auth status
./build/inngest functions list
./build/inngest runs list --status COMPLETED --since 1h
./build/inngest events send test/user.signup --data '{"userId": "123"}'
./build/inngest dev status
```

## Structure

```
cmd/inngest/main.go                  # entry point, ldflags: version
internal/cli/root.go                 # root Cobra command, --env/--output/--dev flags
internal/cli/commands/               # one file per command group
internal/inngest/                    # Inngest API client (GraphQL + REST + dev server)
internal/common/config/config.go     # config load/save, env var fallbacks
pkg/output/output.go                 # JSON / text / table formatter
scripts/release.sh                   # cross-platform release builder
```

## Detailed Rules

@.claude/rules/go.md
@.claude/rules/security.md
@.claude/rules/release.md

## Environment

- Platform: darwin
