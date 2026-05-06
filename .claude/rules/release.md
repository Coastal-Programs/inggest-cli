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

### Version Files — ALL 6 must match

When bumping, use the helper (updates all 6 atomically):
```bash
./scripts/bump-version.sh X.Y.Z
```

Files managed:
1. `package.json` — `version` AND all `optionalDependencies` versions
2. `npm/darwin-arm64/package.json`
3. `npm/darwin-x64/package.json`
4. `npm/linux-x64/package.json`
5. `npm/linux-arm64/package.json`
6. `npm/windows-x64/package.json`

### CHANGELOG.md format

The CHANGELOG entry is extracted by CI and becomes the GitHub Release description — write it well.

Each version block must have:
- A one or two sentence **summary** at the top describing what this release is about
- `### Added` — new commands, features, endpoints
- `### Fixed` — bug fixes
- `### Security` — any security improvements (if applicable)
- `### Changed` — breaking changes or behaviour changes (if applicable)

Example:
```markdown
## [0.3.0] - 2026-03-01

This release adds run-replay and backlog commands, and fixes GraphQL pagination.

### Added
- `inngest runs replay` — replay a function run
- `inngest backlog` — show queued and running runs per function

### Fixed
- GraphQL cursor pagination no longer skips the last page
```

### Release Steps

1. Write `CHANGELOG.md` entry (see format above)
2. `./scripts/bump-version.sh X.Y.Z` — bumps all 6 package.json files
3. `make release` — cross-compiles binaries AND copies them into `npm/<platform>/` dirs
4. (Optional local publish) `cd npm/darwin-arm64 && npm publish --access restricted && cd ../..` etc.
5. `git add package.json npm/*/package.json CHANGELOG.md`
6. `git commit -m "chore: release vX.Y.Z"`
7. `git tag -a vX.Y.Z -m "Release vX.Y.Z"` — annotated tags only, never lightweight
8. `git push --follow-tags`

Pushing the tag triggers `.github/workflows/release.yml`, which cross-compiles, copies binaries, publishes all 6 npm packages (`--access restricted`), and creates the GitHub Release.

> Never push a tag without a well-written CHANGELOG entry first.

### npm Publishing Notes

- **Token type**: MUST use Classic Automation token (not Granular) — granular tokens cannot create new scoped packages
- **2FA bypass**: Token must have "Bypass 2FA" enabled
- **Publish order**: Platform packages FIRST, root wrapper LAST
- **GitHub secret**: `NPM_TOKEN` already set in repo; refresh at https://www.npmjs.com/settings/jakeschepis/tokens
- **Access**: all packages publish with `--access restricted` (private scoped)
