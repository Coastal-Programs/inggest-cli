# Scaffolding Cleanup — inngest-cli

Resolve every finding from the project audit: GAPs and INFO observations.
This is scaffolding/tooling work only — no source-code refactors.

## Goal

Bring project scaffolding to a clean, conventional state: cross-editor consistency,
no stale tracked artifacts, complete agent docs, a non-mutating verify gate, and a
pre-commit hook. Match real-world Go-CLI conventions and official tool docs.

## Findings being addressed

| # | Finding | Severity | Action |
|---|---------|----------|--------|
| 1 | `.editorconfig` missing | GAP | Add `.editorconfig` |
| 2 | `cover.out` + `coverage.out` committed but gitignored | INFO | Untrack from git |
| 3 | `AGENTS.md` missing | GAP | Add `AGENTS.md` |
| 4 | No pre-commit hook | GAP | Add `lefthook.yml` (lightweight, no Node dep) |
| 5 | `make check` mutates files instead of verifying | INFO | Add non-mutating `fmt-check` target; make `check` use it |
| 6 | CI generates coverage but never enforces/uploads it | INFO | Add coverage summary step to `test.yml` |
| 7 | 11.5MB `inngest` binary in repo root | INFO | Confirm untracked; leave on disk (gitignored already) |
| 8 | slog INFO from audit | INFO | Resolved: no logging in codebase — CLI uses `pkg/output`. No action. |

## Details & decisions

### 1. `.editorconfig` (new file, additive)
Go uses hard tabs (gofmt); YAML/JSON/JS/Markdown/shell use 2-space soft tabs.
Per editorconfig.org spec: `root = true`, `charset = utf-8`, `end_of_line = lf`,
`insert_final_newline = true`, `trim_trailing_whitespace = true`.
- `*.go` and `Makefile` → `indent_style = tab`.
- `*.{js,json,yml,yaml,md,sh}` → `indent_style = space`, `indent_size = 2`.
- `*.md` → `trim_trailing_whitespace = false` (Markdown hard line breaks).

### 2. Untrack coverage files (git operation, non-destructive — files stay on disk)
`cover.out` and `coverage.out` are listed in `.gitignore` but were committed before
the ignore rule existed. Task must first run `git ls-files` to CONFIRM they are
tracked; if tracked, `git rm --cached cover.out coverage.out` (keeps working-tree
copies). If `git ls-files` shows they are NOT tracked, skip — no action needed.
Do not delete the files from disk.

### 3. `AGENTS.md` (new file, additive)
README markets the tool "for AI Agents"; `AGENTS.md` is the emerging cross-tool
convention (agents.md). `CLAUDE.md` already exists and is the canonical agent doc.
Create `AGENTS.md` as a thin pointer file that references `CLAUDE.md` plus the
`make` commands and structure summary, so non-Claude tools get the same context.
Keep it short — do not duplicate/fork content that will drift. Verify the AGENTS.md
convention against https://agents.md before finalizing wording.

### 4. Pre-commit hook — `lefthook.yml` (new file, additive)
Choice: **lefthook**, not Husky (Husky needs a Node dev-dependency + `node_modules`;
this is a Go project with a deliberately dependency-free npm wrapper). Lefthook is a
single Go binary, fits the stack, and is the common choice for Go projects.
- `pre-commit`: run `gofmt -l` (fail if output non-empty) and `go vet ./...` on
  staged Go files.
- `pre-push`: run `go test ./...`.
- The hook config is committed; installation (`lefthook install`) is opt-in and
  documented in README + a `make hooks` target. Do not auto-install hooks.
Verify lefthook config schema against https://github.com/evilmartians/lefthook docs
before finalizing.

### 5. Non-mutating format check in Makefile (edit `Makefile`)
Current `fmt` runs `go fmt ./...` which rewrites files; `check` depends on it, so the
gate mutates the tree. Add a `fmt-check` target that runs `gofmt -l .` and fails if
output is non-empty (the canonical non-mutating check, also used by `golangci-lint`).
Repoint `check` to `fmt-check vet lint` so the gate is read-only. Keep `fmt` as the
explicit mutating fixer. Update the `.PHONY` line and the `## check:` help comment.

### 6. CI coverage enforcement (edit `.github/workflows/test.yml`)
The test job already produces `coverage.out` but discards it. Add a step after the
test step that prints a coverage summary via `go tool cover -func=coverage.out`
(stdlib, no external action, no token). This surfaces total coverage in CI logs
without adding a third-party dependency or a coverage-gate service. Do not add
Codecov or fail-on-threshold — that is a policy decision left to the user.

### 7. Root `inngest` binary
Already gitignored (`/inngest` in `.gitignore`). The untrack task (#2) will also
confirm via `git ls-files` that `inngest` is not tracked. If it IS tracked,
`git rm --cached inngest`. No disk deletion.

## Risks

- Untracking files (#2): safe — `--cached` keeps working-tree copies; files remain
  gitignored so they will not be re-added.
- `make check` change (#5): `fmt-check` will FAIL the build if any file is unformatted.
  Task must run `gofmt -l .` first and, if it reports files, run `gofmt -w` to format
  them so the new gate passes — formatting-only changes, no logic edits.
- lefthook (#4): config-only; hooks are not installed automatically, so no contributor
  workflow breaks until they opt in.
- All other items are new files or additive — no overwrites of existing content.

## Verification

After changes, run from project root:
- `gofmt -l .` → must print nothing.
- `go vet ./...` → must pass.
- `make fmt-check` → must pass (new target).
- `make check` → must pass end-to-end (now non-mutating).
- `go test ./...` → must pass.
- Confirm `git status` shows `cover.out`/`coverage.out` removed from index but still
  present on disk and ignored.
- Validate `.editorconfig`, `lefthook.yml`, and `AGENTS.md` against their official
  specs/docs (editorconfig.org, lefthook repo, agents.md) before marking done.

## Steps

1. Create `.editorconfig` at project root per the editorconfig.org spec: `root = true`, UTF-8/LF/final-newline/trim-trailing-whitespace defaults, tab indent for `*.go` and `Makefile`, 2-space indent for `*.{js,json,yml,yaml,md,sh}`, and `trim_trailing_whitespace = false` for `*.md`.
2. Confirm with `git ls-files` whether `cover.out`, `coverage.out`, and `inngest` are tracked; for any that are, run `git rm --cached <file>` to untrack while keeping the working-tree copy — never delete from disk.
3. Create `AGENTS.md` at project root as a concise pointer to `CLAUDE.md`, the `make` command table, and the structure summary; verify the convention/wording against https://agents.md first.
4. Create `lefthook.yml` at project root with a `pre-commit` stage (`gofmt -l` check + `go vet ./...` on staged Go files) and a `pre-push` stage (`go test ./...`); verify the config schema against the official lefthook docs.
5. Edit the `Makefile`: add a non-mutating `fmt-check` target running `gofmt -l .` (fail if output non-empty), add a `hooks` target running `lefthook install`, repoint `check` to depend on `fmt-check vet lint`, and update the `.PHONY` line plus `## ` help comments.
6. Edit `.github/workflows/test.yml` to add a step after the test step that runs `go tool cover -func=coverage.out` to print a coverage summary in CI logs.
7. Update `README.md` to document the new `make hooks` / pre-commit setup and the `make fmt-check` target so contributors can opt into hooks.
8. Run `gofmt -w` on any files `gofmt -l .` flags, then verify the full pipeline: `gofmt -l .`, `go vet ./...`, `make fmt-check`, `make check`, `go test ./...`, and confirm `git status` shows the coverage files untracked but still on disk.
