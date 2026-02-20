#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:-dev}"
BINARY="xero"
CMD="./cmd/xero"
DIST="./dist"

# CLIENT_ID and PROXY_URL are injected from CI secrets (GitHub Actions env vars)
LDFLAGS="-s -w \
  -X main.version=${VERSION} \
  -X main.defaultClientID=${CLIENT_ID:-} \
  -X main.proxyURL=${PROXY_URL:-}"

mkdir -p "${DIST}"

PLATFORMS=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS="${PLATFORM%/*}"
  GOARCH="${PLATFORM#*/}"
  OUTPUT="${DIST}/${BINARY}-${VERSION}-${GOOS}-${GOARCH}"

  if [ "${GOOS}" = "windows" ]; then
    OUTPUT="${OUTPUT}.exe"
    ARCHIVE="${DIST}/${BINARY}-${VERSION}-${GOOS}-${GOARCH}.zip"
  else
    ARCHIVE="${DIST}/${BINARY}-${VERSION}-${GOOS}-${GOARCH}.tar.gz"
  fi

  echo "Building ${GOOS}/${GOARCH}..."
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" "${CMD}"

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
