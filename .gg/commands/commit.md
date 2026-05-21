---
name: commit
description: Run checks, commit, push, and create a versioned release tag that triggers npm publishing via CI
---

## CRITICAL: This is a PRIVATE package

- `@coastal-programs/inggest` and all platform sub-packages are **private/restricted** npm packages — that is what the paid npm plan is for.
- **NEVER** change `npm publish --access restricted` to `--access public` in `.github/workflows/release.yml`. Publishing public would leak a private CLI.
- A `404 Not Found` on `PUT` during publish is **always a token problem**, never an `--access` problem. Do not "fix" it by changing access.

## CRITICAL: npm Token Requirements

- The `NPM_TOKEN` repo secret **must** be a **Classic Automation** token (not Granular).
- Classic Automation tokens bypass 2FA — without this, CI publish fails with OTP errors.
- Granular tokens return 404 on PUT for new packages.
- If expired or missing: https://www.npmjs.com/settings/jakeschepis/tokens → "Generate New Token" → **Classic** → **Automation**.
- Token rotation is the **user's** responsibility — surface it as a blocker and stop; never attempt to work around it.

## Autonomy

Run this whole process end-to-end **without asking the user to confirm** any
of the following — the decisions below are already made:

- Whether to commit, bump, tag, and push — yes, always, once checks pass.
- The version number — computed mechanically (see Step 5), never a judgement call.
- Which files to stage — the changed files relevant to the work.

Only stop and ask the user when:

- A pre-flight check (`go vet`, `go test -race`) fails and the fix is non-obvious.
- The working tree is not on `main`, or has unrelated uncommitted changes.
- A genuine prerequisite is missing that only the user can supply (e.g. the
  `NPM_TOKEN` secret needs rotating). Surface it as a blocker, do not guess.

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

**Versioning scheme (fixed — no judgement, no asking):**

- The version is always `0.2.<N>` — the `0.2` prefix is permanent.
- `<N>` is a single ever-incrementing integer. The next release after
  `0.2.21` is `0.2.22`, then `0.2.23`, and so on.
- There is **no** patch/minor/major decision. Just increment `<N>` by 1.

Compute `NEXT_VERSION` by taking the current `0.2.<N>` and incrementing `<N>`:

```bash
CURRENT=$(grep '"version"' package.json | head -1 | sed -E 's/.*"([0-9.]+)".*/\1/')
N=$(echo "$CURRENT" | cut -d. -f3)
NEXT_VERSION="0.2.$((N + 1))"
echo "Bumping ${CURRENT} -> ${NEXT_VERSION}"
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
