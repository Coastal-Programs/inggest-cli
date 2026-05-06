#!/usr/bin/env node
"use strict";

const { platform, arch } = process;

// Map Node's platform/arch to the npm sub-package names we publish.
const PLATFORMS = {
  darwin: {
    arm64: "@coastal-programs/inggest-darwin-arm64/inngest",
    x64: "@coastal-programs/inggest-darwin-x64/inngest",
  },
  linux: {
    x64: "@coastal-programs/inggest-linux-x64/inngest",
    arm64: "@coastal-programs/inggest-linux-arm64/inngest",
  },
  win32: {
    x64: "@coastal-programs/inggest-windows-x64/inngest.exe",
  },
};

const binaryModulePath =
  process.env.INNGEST_BINARY || PLATFORMS?.[platform]?.[arch];

if (!binaryModulePath) {
  console.error(
    `The Inngest CLI does not have a prebuilt binary for your platform (${platform}/${arch}).\n` +
      `Install Go and build from source: go install github.com/Coastal-Programs/inggest-cli/cmd/inngest@latest`
  );
  process.exit(1);
}

let binaryPath;
try {
  binaryPath = require.resolve(binaryModulePath);
} catch {
  console.error(
    `Could not find the Inngest binary for ${platform}/${arch}.\n` +
      `The optional package "${binaryModulePath.split("/").slice(0, -1).join("/")}" may not have been installed.\n` +
      `Try: npm install`
  );
  process.exit(1);
}

const { spawnSync } = require("child_process");
const result = spawnSync(binaryPath, process.argv.slice(2), {
  shell: false,
  stdio: "inherit",
});

if (result.error) {
  throw result.error;
}

process.exitCode = result.status ?? 1;
