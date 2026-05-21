---
name: commit
description: Run checks, commit, push, and create a versioned release tag that triggers npm publishing via CI
---

## CRITICAL: This is a PRIVATE package

- `@coastal-programs/inggest` and all platform sub-packages are **private/restricted** npm packages — that is what the paid npm plan is for.
- **NEVER** change `npm publish --access restricted` to `--access public` in `.github/workflows/release.yml`. Publishing public would leak a private CLI.
- A `404 Not Found` on `PUT` during publish is **always a token problem**, never an `--access` problem. Do not "fix" it by changing access.

## CRITICAL: npm Token Requirements

- The `NPM_TOKEN` secret is a single **Granular Access Token** shared across all
  `@coastal-programs` CLI repos (`inggest-cli`, `notion-cli`, `sageo-cli`).
- It must be configured as: **Packages and scopes = Read and write**, scope
  **`@coastal-programs`**, **Bypass 2FA = on**, **no IP range restriction**.
- A `404 Not Found` on `PUT` during publish means the token lacks **write**
  scope on `@coastal-programs` — fix the token's permission, NOT `--access`.
- A `403 Forbidden` means the secret is missing/expired in the repo.
- npm no longer offers "Classic" tokens — Granular with the scope above is the
  supported setup. A correctly-scoped Granular token publishes fine.
- Token rotation is the **user's** responsibility — surface it as a blocker and
  stop; never attempt to work around it. Never paste a token into chat or a file.
- Set the secret across all three repos at once:
  `gh secret set NPM_TOKEN --org Coastal-Programs --repos inggest-cli,notion-cli,sageo-cli`

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
make check          # fmt-check + vet + lint
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

### If the release run FAILS

The tag is already consumed and npm rejects re-publishing the same version.
Recover by shipping the next number — do NOT reuse the tag:

```bash
git tag -d "v${NEXT_VERSION}"                       # delete local tag
git push origin ":refs/tags/v${NEXT_VERSION}"       # delete remote tag
```

Then fix the root cause (usually the `NPM_TOKEN` secret), re-run Step 5
onward to bump to the next `0.2.<N>`, and tag again.

---

## Step 8 — Verify (after CI completes)

The **CI release run is the authoritative source of truth** — not `npm view`.

```bash
# Watch the release run to completion
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status

# Confirm each publish step succeeded (look for "+ @coastal-programs/...@<version>")
gh run view <run-id> --json jobs --jq '.jobs[].steps[] | "\(.conclusion)\t\(.name)"'

# Confirm GitHub Release
gh release view "v${NEXT_VERSION}"
```

> **Note:** `npm view @coastal-programs/inggest version` often returns `E404`
> for several minutes after a successful publish — private/restricted packages
> lag in npm's read cache. A 404 from `npm view` right after a green CI run is
> NOT a failure. Trust the CI publish log (`+ @coastal-programs/...@<version>`).

---

## Troubleshooting

| Problem | Cause | Fix |
|---------|-------|-----|
| CI `npm publish` → 404 on PUT | Token lacks write scope on `@coastal-programs` | Regenerate Granular token with Packages = Read and write, scope `@coastal-programs` |
| CI `npm publish` → 403 Forbidden | `NPM_TOKEN` secret missing or expired | Re-set the secret across all three repos via `gh secret set` |
| CI `npm publish` → OTP required | Token's "Bypass 2FA" not enabled | Regenerate Granular token with Bypass 2FA on |
| `npm view` returns 404 after green CI | Private-package read-cache lag | Not a failure — trust the CI publish log; recheck in a few minutes |
| Version mismatch after install | A platform `package.json` not bumped | Rerun `./scripts/bump-version.sh 0.2.<N>` |
| Release notes blank | CHANGELOG heading format wrong | Must be exactly `## [X.Y.Z] - YYYY-MM-DD` |
| Release run failed, tag stuck | Tag consumed, npm rejects re-publish | Delete the tag (see Step 7), bump to next `0.2.<N>`, re-tag |
