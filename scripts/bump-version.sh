#!/usr/bin/env bash
# bump-version.sh <version>
# Synchronises the npm version across the root package and all platform
# sub-packages so they match a given release version (e.g. "0.2.0").
# Run this BEFORE tagging a release. Commit the result.
#
# Usage:
#   ./scripts/bump-version.sh 0.2.0
set -euo pipefail

VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

# Validate semver-ish (digits + dots, optional pre-release)
if ! echo "${VERSION}" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]'; then
  echo "Error: version must be semver (e.g. 1.2.3)" >&2
  exit 1
fi

echo "Bumping npm packages to ${VERSION}..."

npm version "${VERSION}" --no-git-tag-version --allow-same-version

for dir in npm/darwin-arm64 npm/darwin-x64 npm/linux-x64 npm/linux-arm64 npm/windows-x64; do
  (cd "${dir}" && npm version "${VERSION}" --no-git-tag-version --allow-same-version)
done

# Keep optionalDependencies versions in sync
node -e "
  const fs = require('fs');
  const pkg = JSON.parse(fs.readFileSync('package.json', 'utf8'));
  for (const k of Object.keys(pkg.optionalDependencies)) {
    pkg.optionalDependencies[k] = '${VERSION}';
  }
  fs.writeFileSync('package.json', JSON.stringify(pkg, null, 2) + '\n');
"

echo "Done. Commit the changes and tag: git tag v${VERSION}"
