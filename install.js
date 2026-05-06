"use strict";

// Postinstall script: validate that the platform-specific optional dependency
// was installed. This is a soft check — npm already handles downloading the
// right optionalDependency, so we only warn if nothing landed.

const { platform, arch } = process;

const PACKAGES = {
  darwin: { arm64: "@coastal-programs/inggest-darwin-arm64", x64: "@coastal-programs/inggest-darwin-x64" },
  linux: { x64: "@coastal-programs/inggest-linux-x64", arm64: "@coastal-programs/inggest-linux-arm64" },
  win32: { x64: "@coastal-programs/inggest-windows-x64" },
};

const pkg = PACKAGES?.[platform]?.[arch];

if (!pkg) {
  // Unsupported platform — not an error, just skip.
  process.exit(0);
}

try {
  require.resolve(`${pkg}/package.json`);
} catch {
  // optionalDependencies are allowed to fail to install — don't hard-fail here.
  // The wrapper (bin/inngest.js) will surface a useful error at runtime.
  console.warn(
    `[inngest] Warning: optional package "${pkg}" was not installed.\n` +
      `  Your platform (${platform}/${arch}) may not be supported, or the package registry may be unreachable.\n` +
      `  You can override the binary path with INNGEST_BINARY=<path-to-binary>.`
  );
}
