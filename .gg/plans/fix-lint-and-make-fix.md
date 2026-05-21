# Plan: Fix lint warnings + align the `fix` workflow with `make check`

## Background — what `make check` is and why it fails

`make check` is the project's **non-mutating pre-commit gate**. It runs
`fmt-check` + `vet` + `lint`, where `lint` is `golangci-lint run ./...`.
It is "non-mutating" — it only *reports* problems, never edits files.

`.golangci.yml` enables the `modernize` linter (a `gopls`-based analyzer that
flags old-style code that has a cleaner modern equivalent). It currently
reports **3 warnings**, all in `internal/inngest/client_test.go`:

```
client_test.go:334  var calls int64  → can be atomic.Int64
client_test.go:409  var calls int64  → can be atomic.Int64
client_test.go:541  var calls int64  → can be atomic.Int64
```

These are **not bugs** — the tests pass. They are modernization suggestions:
Go 1.19+ added typed atomics (`atomic.Int64`) that are safer than a bare
`int64` manipulated via `atomic.AddInt64(&x, ...)` / `atomic.LoadInt64(&x)`.

### Why only 3 of ~14 `var calls int64` are flagged

`grep` finds ~14 `var calls int64` declarations in that file. `modernize`
only flags 3 because it conservatively flags the ones it can mechanically
prove are *only* used through `atomic.*Int64(&calls, ...)` helpers. The
other declarations are flagged-or-not based on usage shape; to keep the
file internally consistent and prevent the warning from reappearing when
code is reshuffled, this plan converts **all `var calls int64` (and the one
`callCount := int64(0)`) in this file** to `atomic.Int64`, not just the 3.

### Real-world confirmation

- `var calls atomic.Int64` + `calls.Add(1)` + `calls.Load()` is the
  standard modern pattern — verified across `openpcc/openpcc`,
  `coder/envbuilder`, `WangYihang/Platypus`, `KubeElasti`, and others
  (kencode-search, 8+ repos).
- `golangci-lint run --fix` is the canonical mutating fix command, paired
  with plain `golangci-lint run` for the check — same tool for both.
  Confirmed in `nektos/act`, `usememos/memos`, `tmuxpack/tpack`,
  `BackupTime/clash`, `gleanwork/glean-cli` (kencode-search).

## Root cause of "we keep missing these"

`.gg/commands/fix.md` runs **`staticcheck`** as its linter. `make check`
runs **`golangci-lint`**. They are *different tools with different rule
sets* — `staticcheck` has no `modernize` analyzer, so `fix` can never catch
a `modernize` warning that `check` reports. The fix workflow and the check
gate must use the **same linter** or they will permanently disagree.

## npm registry status

- `https://registry.npmjs.org/@coastal-programs/inggest` → **HTTP 404**
- `https://registry.npmjs.org/@coastal-programs/inggest-darwin-arm64` → **HTTP 404**

The unauthenticated public registry returns 404 for *both* unpublished
packages **and** private/restricted packages it won't reveal anonymously.
`package.json` and all 5 platform `package.json` files set
`"publishConfig": { "access": "restricted" }`, and `release.yml` publishes
with `npm publish --access restricted`. So the package is **configured as
private**; whether a release has ever actually run cannot be proven from
the public registry alone.

This plan does **not** change public/private status — that is a
business/billing decision for the user. It only documents the current
state. If the user later wants it public, the change is: set
`"access": "public"` in all 6 `package.json` files and change
`--access restricted` → `--access public` in `release.yml`. That is called
out here but intentionally **not** included in the steps.

## What this plan does NOT touch

- The `commit`/release flow (`.gg/commands/commit.md`, `release.yml`,
  `scripts/release.sh`, `bump-version.sh`) — already exists and works; out
  of scope for this round per the user.
- Public vs private npm status — documented above, not changed.

---

## Changes

### 1. `internal/inngest/client_test.go` — modernize atomics

For **every** `var calls int64` in the file (~14 sites) and the lone
`callCount := int64(0)` at line ~798:

- `var calls int64` → `var calls atomic.Int64`
- `callCount := int64(0)` → `var callCount atomic.Int64`
- `atomic.AddInt64(&calls, 1)` → `calls.Add(1)`
- `n := atomic.AddInt64(&calls, 1)` → `n := calls.Add(1)`
- `atomic.LoadInt64(&calls)` → `calls.Load()`
- `got := atomic.LoadInt64(&calls)` → `got := calls.Load()`
- `if got := atomic.LoadInt64(&calls); got != 2` → `if got := calls.Load(); got != 2`

`atomic.Int64.Add` and `.Load` both return `int64`, so comparisons like
`got != int64(maxRetries)` and `n == 1` keep working unchanged. The
`sync/atomic` import stays (the package still provides the type). No
behaviour change — only the atomic mechanism is modernized.

Verification after editing: `gofmt -l .` clean, `go vet ./...` passes,
`golangci-lint run ./...` reports 0 issues, `go test -race ./internal/inngest/`
passes.

### 2. `Makefile` — add a mutating `fix` target

`check` stays exactly as-is (non-mutating gate). Add a sibling `fix` target
that is the *mutating* counterpart, mirroring real-world `lint-fix`:

```make
## fix: Auto-fix formatting and lint issues (mutating)
fix:
	gofmt -w .
	@which golangci-lint > /dev/null 2>&1 || go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.3
	golangci-lint run --fix ./...
```

- `gofmt -w .` — applies formatting (mutating counterpart of `fmt-check`).
- `golangci-lint run --fix` — same linter as `check`'s `lint` target, with
  autofix on. Because it is the *same tool* as `check`, anything `fix`
  cannot auto-resolve is exactly what `check` will still report — no more
  silent gaps between the two.
- Add `fix` to the `.PHONY` line.
- Place the `fix` target directly after `fmt-check` so related targets sit
  together; keep `check` unchanged.

Note: `golangci-lint run --fix` resolves what each linter can autofix
(`gofmt`, `goimports`, and `modernize`'s mechanical rewrites). Step 1
applies the 3 `modernize` fixes by hand so the tree is already green and
does not depend on autofix behaviour for correctness.

### 3. `.gg/commands/fix.md` — use `golangci-lint`, not `staticcheck`

Rewrite the command doc so the `fix` workflow uses the **same linter as
`make check`**. This is the change that stops warnings slipping through.

- Step 1 "Run Checks": replace `staticcheck ./...` with
  `golangci-lint run ./...`; keep `gofmt -l .`, `go vet ./...`,
  `go test ./... -count=1`. Add a note that `make fix` auto-fixes the
  mechanical subset first.
- Step 2 "Collect and Group Errors": rename the "Lint errors" domain source
  from `staticcheck` to `golangci-lint`.
- Step 3 "Spawn Parallel Agents": before spawning agents, run `make fix`
  (`gofmt -w .` + `golangci-lint run --fix`) to clear everything
  auto-fixable; only spawn agents for the issues that remain.
- Step 4 "Verify": re-run `golangci-lint run ./...` (not `staticcheck`) plus
  `go vet` and `go test`, i.e. the same set `make check` uses.

### 4. `AGENTS.md` + `CLAUDE.md` — document `make fix`

`AGENTS.md` "Build & test commands" lists `fmt`, `fmt-check`, `vet`, `lint`,
`check` but not `fix`. Add one line:

```
make fix         # gofmt -w + golangci-lint --fix (mutating auto-fixer)
```

placed right after the `make fmt` line. `CLAUDE.md`'s Commands block is
terse (only 4 targets) — leave it unless trivially natural; `AGENTS.md` is
the canonical command reference per its own header.

### 5. `README.md` — mention `make fix` in Contributing

The Contributing section lists `make check`, `fmt-check`, `fmt`, `test`,
`build`, `hooks`. Add `make fix` next to `make fmt` with a one-line
description (`auto-fix formatting + lint issues`).

---

## Risks

- **Behaviour change in tests:** none expected — `atomic.Int64.Add/Load`
  return `int64` and are drop-in for the helper calls. The `-race`
  detector run is the guard; if any comparison breaks, `go vet`/compile
  catches it before tests even run.
- **`golangci-lint` version drift:** `make fix` pins the same
  `v2.11.3` install fallback as `make lint`, so `fix` and `check` use an
  identical linter version.
- **`--fix` touching unrelated files:** `golangci-lint run --fix` only
  rewrites what its formatters/linters can safely autofix; run on a clean
  tree so any change is reviewable in `git diff`. Step 1 is applied
  manually first so the documented 3 warnings are not left to autofix.
- **npm status:** untouched. The 404 is documented, not acted on. No
  release is triggered by this plan.

## Verification (run after all steps)

1. `gofmt -l .` → empty
2. `go vet ./...` → passes
3. `golangci-lint run ./...` → **0 issues** (the 3 modernize warnings gone)
4. `go test -race ./...` → all packages pass
5. `make check` → passes end-to-end (this is the original goal)
6. `make fix` → runs clean, leaves a green tree (no diff on an
   already-fixed tree)

## Steps

1. Edit `internal/inngest/client_test.go`: convert every `var calls int64`
   to `var calls atomic.Int64`, convert `callCount := int64(0)` to
   `var callCount atomic.Int64`, and rewrite all `atomic.AddInt64(&x, …)` /
   `atomic.LoadInt64(&x)` call sites to the `x.Add(…)` / `x.Load()` method
   form; keep the `sync/atomic` import.
2. Run `gofmt -l .`, `go vet ./...`, `golangci-lint run ./...`, and
   `go test -race ./internal/inngest/` to confirm the 3 warnings are gone
   and tests still pass.
3. Edit `Makefile`: add a mutating `fix` target (`gofmt -w .` +
   `golangci-lint run --fix ./...` with the pinned install fallback) after
   `fmt-check`, and add `fix` to the `.PHONY` list.
4. Rewrite `.gg/commands/fix.md` to use `golangci-lint run` instead of
   `staticcheck`, reference `make fix` for the auto-fixable subset, and make
   the verify step re-run `golangci-lint`/`vet`/`test`.
5. Edit `AGENTS.md`: add a `make fix` line to the Build & test commands
   block after `make fmt`.
6. Edit `README.md`: add `make fix` to the Contributing commands list next
   to `make fmt`.
7. Run the full Verification list: `gofmt -l .`, `go vet ./...`,
   `golangci-lint run ./...`, `go test -race ./...`, `make check`, and
   `make fix` — confirm `make check` passes end-to-end and `make fix`
   leaves a clean tree.
