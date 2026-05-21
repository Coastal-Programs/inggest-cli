# Fix: published binary reports `version: "dev"` instead of the release version

## Problem

`npm install -g @coastal-programs/inggest` then `inngest version` prints:

```json
{ "arch": "arm64", "os": "darwin", "version": "dev" }
```

It should print `version: "v0.2.22"` (or `0.2.22`). The binary inside the
published npm platform package was compiled **without** the
`-X main.version=...` ldflag, so it fell back to the `var version = "dev"`
default in `cmd/inngest/main.go`.

## Root cause analysis

The version chain is logically correct end to end:

`release.sh $1` → `LDFLAGS -X main.version=$VERSION` → `go build` →
`main.version` → `cli.Execute(version)` → `state.AppVersion` →
`version` command output.

So a `dev` result means the binary was built with the ldflag missing or
empty. Two concrete defects make this possible and undetectable:

1. **Fragile `LDFLAGS` construction in `scripts/release.sh` (lines 9-10).**
   `LDFLAGS` is built with a backslash line-continuation *inside* a
   double-quoted string:

   ```bash
   LDFLAGS="-s -w \
     -X main.version=${VERSION}"
   ```

   This embeds a literal newline + two leading spaces into the value, so
   `LDFLAGS` becomes `-s -w \n  -X main.version=v0.2.22`. It is then passed
   as a single quoted word: `go build -ldflags "${LDFLAGS}"`. Go's linker
   tokenises `-ldflags` on whitespace, so this *often* works, but it is
   brittle: any tooling/quoting change, or an empty `VERSION`, silently
   yields a binary with no version. The Makefile has the *same* pattern
   (lines 6-7).

2. **No build-time or test-time guard.** Nothing fails the release when a
   `dev` binary is produced:
   - `scripts/release.sh` never executes the binary it just built.
   - `internal/cli/integration_test.go` `TestMain` builds the test binary
     with `go build -o ...` and **no ldflags** (line 27), and
     `TestBinary_Version` only asserts the `version` *key exists*, not its
     value (lines 98-102). A `dev` build passes every test.
   - The release workflow publishes to npm without ever running
     `inngest version` against a built artifact.

Contributing factor: `scripts/release.sh` is invoked with the raw tag
(`v0.2.22`, with `v`), while `bump-version.sh` / the workflow's "Sync npm
version" step use the stripped form (`0.2.22`). The embedded version and
the npm `package.json` version therefore differ by a `v` prefix. Not the
cause of the `dev` bug, but an inconsistency to fix while here so
`inngest version` and `npm view` agree.

## Reference (real-world pattern)

GoReleaser-based Go CLIs inject the version with a clean single ldflag
string, e.g. `-s -w -X main.version={{.Version}}` (confirmed across
`gleanwork/glean-cli`, `Telmate/terraform-provider-proxmox`,
`pete911/kubectl-iam4sa`). The fix mirrors that: build the ldflags as a
single-line string with no embedded newline, and verify the artifact.

## Fix design

### A. Make `release.sh` robust and self-verifying

- Build `LDFLAGS` as a single line, no backslash-newline:
  `LDFLAGS="-s -w -X main.version=${VERSION}"`.
- Fail fast if `VERSION` is empty or still the literal `dev` when running
  in CI (treat an unstamped release as a hard error).
- Normalise the version once at the top: accept either `v0.2.22` or
  `0.2.22`, derive both `VERSION` (with `v`, used for ldflags + artifact
  names) and `VERSION_NUM` (no `v`) so the value embedded in the binary is
  deterministic. Decide on ONE canonical embedded form — recommend the
  `v`-prefixed tag, matching `git describe` and the GitHub release tag.
- After building each platform binary for the **host** platform, execute
  it (`<binary> version`) and assert the output contains the expected
  version; abort the release on mismatch. Non-host binaries cannot be run
  on the runner, but verifying the host build catches a broken ldflag for
  all of them (same flags, same toolchain).

### B. Apply the same single-line `LDFLAGS` fix to the `Makefile`

Lines 6-7 use the identical backslash-newline pattern; collapse to one
line so `make build` / `make install` are consistent with `release.sh`.

### C. Close the test gap

- In `internal/cli/integration_test.go` `TestMain`, build the test binary
  with an explicit ldflag, e.g.
  `-ldflags "-X main.version=test-<something>"`, and have
  `TestBinary_Version` assert the `version` value **equals** that injected
  string — not merely that the key exists. This makes the ldflag path a
  tested contract.

### D. Add a CI verification step in `release.yml`

After "Build release binaries" and before publishing, run the freshly
built host (`linux-x64`) binary and assert
`inngest version` reports the tag version. If it reports `dev` or a
mismatch, fail the job so nothing is published. This is the backstop that
would have caught v0.2.22.

### E. Re-release to ship a correctly-stamped binary

The published `0.2.22` packages contain the `dev` binary and npm will not
let that version be overwritten. After A-D land and are verified, cut the
next version per the `0.2.<N>` scheme (`0.2.23`) so users get a binary
that reports the real version. Update `CHANGELOG.md` with a `0.2.23`
entry describing the release-tooling fix.

## Risks / considerations

- **Embedded version format change.** If the embedded version switches
  between `v0.2.22` and `0.2.22`, `version_test.go` (`testVersion =
  "v1.2.3"`) and `root_test.go` already use a `v`-prefixed literal, so
  keeping the `v` prefix for the embedded value is the lower-churn choice.
  `npm view` will still report the non-`v` semver — document that the npm
  package version and the binary's self-reported version differ only by
  the `v`, or strip it in the `version` command for display. Decide
  explicitly; do not leave it ambiguous.
- **Host-only artifact verification.** The runner can only execute the
  `linux-amd64` binary. That is sufficient to prove the ldflag wiring,
  since all platforms share build flags; cross-arch execution via QEMU is
  out of scope.
- **`-trimpath`** is unrelated to ldflags and stays as-is.
- Do not touch `npm publish --access restricted` — unrelated, and
  changing it would leak a private package.

## Verification

1. `make build && ./build/inngest version` — reports the `git describe`
   version, never `dev`.
2. `VERSION=v0.2.23 ./scripts/release.sh v0.2.23` locally — succeeds, and
   the in-script host-binary check passes; tampering `VERSION=dev` makes
   the script abort.
3. `go test -tags integration ./internal/cli/...` — the strengthened
   `TestBinary_Version` passes and fails if the ldflag is removed.
4. `make check` and `go test -race ./...` — all green.
5. After tagging `v0.2.23`: the release workflow's new verification step
   passes; `npm install -g @coastal-programs/inggest` then
   `inngest version` reports `v0.2.23` (or `0.2.23`), not `dev`.

## Steps

1. Rewrite `scripts/release.sh` to build `LDFLAGS` as a single line
   (no backslash-newline), normalise the input tag into `VERSION`
   (canonical `v`-prefixed) and `VERSION_NUM`, and abort if the version is
   empty or `dev`.
2. Add a host-platform verification block to `scripts/release.sh` that
   runs the freshly built host binary's `version` command and aborts the
   release if the reported version does not match the expected value.
3. Collapse the `Makefile` `LDFLAGS` definition (lines 6-7) to a single
   line so `make build`/`make install` use the same robust form.
4. Strengthen `internal/cli/integration_test.go`: build the test binary
   in `TestMain` with an explicit `-ldflags "-X main.version=..."` and
   make `TestBinary_Version` assert the `version` value equals the
   injected string.
5. Add a "Verify version injection" step to `.github/workflows/release.yml`
   after "Build release binaries" and before the publish steps, running
   the built host binary and failing the job on a `dev`/mismatched version.
6. Run `make check`, `go test -race ./...`, and
   `go test -tags integration ./internal/cli/...` and fix any failures.
7. Bump to `0.2.23` via `scripts/bump-version.sh`, add a `0.2.23`
   `CHANGELOG.md` entry for the release-tooling fix, commit, tag
   `v0.2.23`, and push to trigger a corrected release.
8. After CI completes, verify `inngest version` from a fresh
   `npm install -g @coastal-programs/inggest` reports `v0.2.23`, not `dev`.
