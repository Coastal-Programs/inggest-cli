#!/usr/bin/env bash
#
# Regenerate THIRD_PARTY_NOTICES.md from the Go module cache.
#
# The released binary is statically linked, so it embeds these modules. The
# Apache-2.0 and BSD-3-Clause licences both require their notices to travel
# with a binary distribution, which is what this file provides.
#
# Run via: make notices
set -euo pipefail

cd "$(dirname "$0")/.."

OUT="THIRD_PARTY_NOTICES.md"

# Modules actually linked into the binary. Keep in sync with:
#   go version -m build/inngest
MODULES=(
  github.com/spf13/cobra
  github.com/spf13/pflag
  golang.org/x/term
  golang.org/x/sys
  github.com/inconshreveable/mousetrap
)

{
  cat <<'HDR'
# Third-Party Notices

The `inngest` binary is statically linked and embeds the open-source modules
listed below. Their licences require that their copyright notices and licence
terms accompany any distribution of the binary, so the full text of each is
reproduced here.

This file is generated from the Go module cache. Regenerate it with
`make notices` after changing dependencies.

HDR

  for m in "${MODULES[@]}"; do
    dir="$(go list -m -f '{{.Dir}}' "$m")"
    version="$(go list -m -f '{{.Version}}' "$m")"

    if [ -z "${dir}" ] || [ ! -d "${dir}" ]; then
      echo "error: module directory not found for ${m}" >&2
      exit 1
    fi

    license_file="$(find "${dir}" -maxdepth 1 -iname 'LICENSE*' -o -maxdepth 1 -iname 'COPYING*' | head -1)"
    if [ -z "${license_file}" ]; then
      echo "error: no licence file found for ${m} in ${dir}" >&2
      exit 1
    fi

    printf -- '---\n\n## %s\n\nVersion: `%s`\n\n```\n' "${m}" "${version}"
    cat "${license_file}"
    printf '```\n\n'
  done
} > "${OUT}"

echo "Wrote ${OUT} ($(grep -c '^## ' "${OUT}") modules)"
