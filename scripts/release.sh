#!/usr/bin/env bash
# release.sh <version>
# Cross-platform release builder. Accepts either a v-prefixed tag
# ("v0.2.23") or a bare semver ("0.2.23"); the v-prefixed form is the
# canonical value embedded in the binary via -X main.version.
set -euo pipefail

RAW_VERSION="${1:-}"
BINARY="inngest"
CMD="./cmd/inngest"
DIST="./dist"

# Normalise the input tag into canonical forms.
if [[ -z "${RAW_VERSION}" || "${RAW_VERSION}" == "dev" ]]; then
  echo "Error: release version is required and must not be 'dev' (got '${RAW_VERSION}')" >&2
  echo "Usage: $0 <version>   e.g. $0 v0.2.23" >&2
  exit 1
fi

VERSION_NUM="${RAW_VERSION#v}"          # bare semver, e.g. 0.2.23
VERSION="v${VERSION_NUM}"              # canonical v-prefixed, embedded in binary

# Single-line ldflags string — no embedded newline. A backslash-newline
# inside the quoted value embeds a literal newline and is fragile.
LDFLAGS="-s -w -X main.version=${VERSION}"

mkdir -p "${DIST}"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

# Map GOOS/GOARCH → npm sub-package directory name
npm_dir() {
  local goos="$1"
  local goarch="$2"
  case "${goos}/${goarch}" in
    darwin/arm64)  echo "npm/darwin-arm64" ;;
    darwin/amd64)  echo "npm/darwin-x64" ;;
    linux/amd64)   echo "npm/linux-x64" ;;
    linux/arm64)   echo "npm/linux-arm64" ;;
    windows/amd64) echo "npm/windows-x64" ;;
    *) echo "" ;;
  esac
}

# Host platform — the only build that can be executed on this runner.
HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"
HOST_VERIFIED=""

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%/*}"
  GOARCH="${PLATFORM#*/}"
  OUTPUT="${DIST}/${BINARY}-${VERSION}-${GOOS}-${GOARCH}"

  if [ "${GOOS}" = "windows" ]; then
    OUTPUT="${OUTPUT}.exe"
    ARCHIVE="${DIST}/inngest-cli-${VERSION}-${GOOS}-${GOARCH}.zip"
  else
    ARCHIVE="${DIST}/inngest-cli-${VERSION}-${GOOS}-${GOARCH}.tar.gz"
  fi

  echo "Building ${GOOS}/${GOARCH}..."
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" go build -trimpath -ldflags "${LDFLAGS}" -o "${OUTPUT}" "${CMD}"

  # Copy binary into its npm sub-package directory for npm publish
  NPM_DIR="$(npm_dir "${GOOS}" "${GOARCH}")"
  if [ -n "${NPM_DIR}" ]; then
    if [ "${GOOS}" = "windows" ]; then
      cp "${OUTPUT}" "${NPM_DIR}/inngest.exe"
    else
      cp "${OUTPUT}" "${NPM_DIR}/inngest"
      chmod +x "${NPM_DIR}/inngest"
    fi
  fi

  # Verify version injection by running the host-platform binary. All
  # platforms share build flags, so a correct host build proves the
  # ldflag wiring for the cross-compiled binaries too.
  if [[ "${GOOS}" == "${HOST_GOOS}" && "${GOOS}/${GOARCH}" == "${HOST_GOOS}/${HOST_GOARCH}" ]]; then
    echo "Verifying version injection on host (${HOST_GOOS}/${HOST_GOARCH})..."
    REPORTED="$("${OUTPUT}" version -o json | grep -o '"version"[^,}]*' | sed 's/.*: *"\(.*\)".*/\1/')"
    if [[ "${REPORTED}" != "${VERSION}" ]]; then
      echo "Error: built binary reports version '${REPORTED}', expected '${VERSION}'" >&2
      echo "The -X main.version ldflag was not applied correctly; aborting release." >&2
      rm "${OUTPUT}"
      exit 1
    fi
    echo "Version injection OK: ${REPORTED}"
    HOST_VERIFIED="yes"
  fi

  if [ "${GOOS}" = "windows" ]; then
    zip -j "${ARCHIVE}" "${OUTPUT}"
  else
    tar -czf "${ARCHIVE}" -C "${DIST}" "$(basename "${OUTPUT}")"
  fi
  rm "${OUTPUT}"
done

if [[ -z "${HOST_VERIFIED}" ]]; then
  echo "Error: host platform ${HOST_GOOS}/${HOST_GOARCH} is not in PLATFORMS;" >&2
  echo "version injection could not be verified. Aborting release." >&2
  exit 1
fi

echo "Generating checksums..."
cd "${DIST}"
sha256sum ./*.tar.gz ./*.zip 2>/dev/null > checksums.txt || \
  shasum -a 256 ./*.tar.gz ./*.zip > checksums.txt
cd ..

echo ""
echo "Release artifacts in ${DIST}:"
ls -lh "${DIST}"
