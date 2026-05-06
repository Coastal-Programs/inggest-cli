#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
BINARY="inngest"
CMD="./cmd/inngest"
DIST="./dist"

LDFLAGS="-s -w \
  -X main.version=${VERSION}"

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

  if [ "${GOOS}" = "windows" ]; then
    zip -j "${ARCHIVE}" "${OUTPUT}"
  else
    tar -czf "${ARCHIVE}" -C "${DIST}" "$(basename "${OUTPUT}")"
  fi
  rm "${OUTPUT}"
done

echo "Generating checksums..."
cd "${DIST}"
sha256sum ./*.tar.gz ./*.zip 2>/dev/null > checksums.txt || \
  shasum -a 256 ./*.tar.gz ./*.zip > checksums.txt
cd ..

echo ""
echo "Release artifacts in ${DIST}:"
ls -lh "${DIST}"
