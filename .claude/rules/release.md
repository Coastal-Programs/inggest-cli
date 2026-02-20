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

Steps:
1. Move `[Unreleased]` items in `CHANGELOG.md` to the new version + date
2. `git commit -m "chore: release v0.2.0"`
3. `git tag -a v0.2.0 -m "Release v0.2.0"` — annotated tags only, never lightweight
4. `git push origin v0.2.0`

GitHub Actions then builds 5 platform binaries and publishes the GitHub Release automatically.

> Never push a tag without updating CHANGELOG.md first.
