---
name: commit
description: Run checks, commit, push, and create a versioned release tag that triggers npm publishing via CI
---

## CRITICAL: npm Token Requirements

- The `NPM_TOKEN` repo secret **must** be a **Classic Automation** token (not Granular).
- Classic Automation tokens bypass 2FA — without this, CI publish fails with OTP errors.
- Granular tokens return 404 on PUT for new packages.
- If expired or missing: https://www.npmjs.com/settings/jakeschepis/tokens → "Generate New Token" → **Classic** → **Automation**.

---

## Step 1 — Pre-flight Checks

Run ALL checks first. Fix every error before continuing.

```bash
go vet ./...
go test -race ./...
```

Verify you are on `main` with a clean working tree:

```bash
git status
git branch --show-current
```

---

## Step 2 — Review Changes

```bash
git status
git diff --staged
git diff
```

---

## Step 3 — Stage Relevant Files

Stage specific files — never `git add -A`:

```bash
git add <file1> <file2> ...
```

---

## Step 4 — Write a High-Quality Commit Message

Generate a commit message from the actual diff:

- Subject line: one line, starts with `Add`, `Update`, `Fix`, `Remove`, or `Refactor`
- Body: short bullets grouped under `Added:`, `Updated:`, `Fixed:`, `Docs:`. Omit empty sections.
- Bullets name concrete files, functions, or behaviours. No vague text.

```bash
subject="your generated one-line subject"
{
  echo "$subject"
  echo
  echo "Added:"
  echo "- ..."
  echo
  echo "Updated:"
  echo "- ..."
  echo
  echo "Fixed:"
  echo "- ..."
  echo
  echo "Docs:"
  echo "- ..."
} > /tmp/inggest_commit_msg.txt

git commit -F /tmp/inggest_commit_msg.txt
git push
```

---

## Step 5 — Bump Version in ALL Package Files

**All 6 `package.json` files must carry the same version. Missing one causes install failures.**

Read current version:

```bash
grep '"version"' package.json | head -1
```

Decide the bump type:
- **patch** (0.1.0 → 0.1.1): bug fixes only
- **minor** (0.1.0 → 0.2.0): new features, backwards compatible
- **major** (0.1.0 → 1.0.0): breaking changes

Compute `NEXT_VERSION` (e.g. `0.1.1`), then run the helper script:

```bash
NEXT_VERSION="0.1.1"   # ← set this
./scripts/bump-version.sh "${NEXT_VERSION}"
```

The script updates all 6 files atomically:
1. `package.json` (root — `version` AND all `optionalDependencies` versions)
2. `npm/darwin-arm64/package.json`
3. `npm/darwin-x64/package.json`
4. `npm/linux-arm64/package.json`
5. `npm/linux-x64/package.json`
6. `npm/windows-x64/package.json`

Verify all six match:

```bash
grep '"version"' package.json npm/*/package.json
```

---

## Step 6 — Update CHANGELOG.md

Add a new section at the top (below `# Changelog`):

```markdown
## [X.Y.Z] - YYYY-MM-DD

### Added/Changed/Fixed
- Description of changes
```

The CI release workflow (`release.yml`) extracts this section as GitHub Release notes — the heading format `## [X.Y.Z]` must match exactly.

---

## Step 7 — Commit Version Bump and Tag

```bash
git add package.json npm/darwin-arm64/package.json npm/darwin-x64/package.json \
        npm/linux-arm64/package.json npm/linux-x64/package.json npm/windows-x64/package.json \
        CHANGELOG.md
git commit -m "chore: release v${NEXT_VERSION}"
git tag -a "v${NEXT_VERSION}" -m "Release v${NEXT_VERSION}"
git push --follow-tags
```

Pushing the tag triggers `.github/workflows/release.yml`, which:

1. Cross-compiles 5 binaries via `scripts/release.sh`
2. Copies binaries into each `npm/<platform>/` directory
3. Publishes the 5 platform packages to npm (`--access restricted`)
4. Publishes the root `@coastal-programs/inggest` wrapper package
5. Creates a GitHub Release with `dist/*` artefacts and CHANGELOG notes

---

## Step 8 — Verify (after CI completes)

```bash
# Check CI
gh run list --limit 3

# Confirm npm versions
npm view @coastal-programs/inggest version
npm view @coastal-programs/inggest-darwin-arm64 version
npm view @coastal-programs/inggest-linux-x64 version

# Confirm GitHub Release
gh release view "v${NEXT_VERSION}"
```

---

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| CI `npm publish` → 404 on PUT | Granular token + new package | Use Classic Automation token |
| CI `npm publish` → 403 Forbidden | Token missing or wrong scope | Update `NPM_TOKEN` secret in repo settings |
| CI `npm publish` → OTP required | Token doesn't bypass 2FA | Create new Classic Automation token with "Bypass 2FA" |
| Version mismatch after install | A platform `package.json` not bumped | Rerun `./scripts/bump-version.sh X.Y.Z` |
| `NPM_TOKEN` secret expired | Old token | Regenerate at npmjs.com → update GitHub secret |
| Release notes blank | CHANGELOG heading format wrong | Must be exactly `## [X.Y.Z] - YYYY-MM-DD` |
