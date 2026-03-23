# Git & Release

## Commit Messages (Conventional Commits)

```
feat: add aged-payables report command
fix: correct 1-indexed pagination in bank transactions
docs: update README installation section
chore: bump cobra to v1.9.0
refactor: extract pagination helper into client.go
ci: add arm64 build target
```

Format: `type: description` — imperative, lowercase, no trailing period

| Type | Semver | When |
|------|--------|------|
| `feat` | MINOR | New command or behaviour |
| `fix` | PATCH | Bug fix |
| `feat!` / `BREAKING CHANGE:` | MAJOR | Incompatible change |
| `docs`, `chore`, `refactor`, `test`, `ci` | none | Everything else |

## Branches
- `main` — always stable, always releasable
- `feat/add-budgets-command`
- `fix/pagination-bank-transactions`

## Before Every Commit
```bash
make check   # fmt + vet + test
```

## Cutting a Release

Semver: `vMAJOR.MINOR.PATCH`
- Bug fix → PATCH (`v0.1.1`)
- New command → MINOR (`v0.2.0`)
- Breaking flag/output change → MAJOR (`v1.0.0`)

### CHANGELOG.md format

The CHANGELOG entry becomes the GitHub Release description automatically — write it well.
Each version block must have:
- A one or two sentence **summary** at the top describing what this release is about
- `### Added` — new commands, features, endpoints
- `### Fixed` — bug fixes
- `### Security` — any security improvements (if applicable)
- `### Changed` — breaking changes or behaviour changes (if applicable)

Example:
```markdown
## [0.3.0] - 2026-03-01

This release adds budget reporting commands and improves error messages across all commands.

### Added
- `inngest runs replay` — replay a function run
- `inngest backlog` — show queued and running runs per function

### Fixed
- GraphQL cursor pagination no longer skips the last page
```

### Steps
1. Write `CHANGELOG.md` entry with summary + categorised changes (see format above)
2. `git commit -m "chore: release v0.3.0"`
3. `git tag -a v0.3.0 -m "Release v0.3.0"` — annotated tags only, never lightweight
4. `git push origin v0.3.0`

GitHub Actions extracts the CHANGELOG entry and uses it as the release description, then builds 5 platform binaries automatically.

> Never push a tag without a well-written CHANGELOG entry first.
ry first.
try first.
