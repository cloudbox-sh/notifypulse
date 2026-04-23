#!/usr/bin/env node
// Meta-package shim: resolves the platform-specific @cloudboxsh/notifypulse-*
// package at runtime and execs the binary it ships. Matches the
// optionalDependencies pattern used by esbuild, biome, bun, swc, etc.

const { spawnSync } = require("node:child_process");
const { existsSync } = require("node:fs");
const path = require("node:path");

const { platform, arch } = process;

const PLATFORM_MAP = {
  "linux-x64": { pkg: "@cloudboxsh/notifypulse-linux-x64", bin: "notifypulse" },
  "linux-arm64": { pkg: "@cloudboxsh/notifypulse-linux-arm64", bin: "notifypulse" },
  "darwin-x64": { pkg: "@cloudboxsh/notifypulse-darwin-x64", bin: "notifypulse" },
  "darwin-arm64": { pkg: "@cloudboxsh/notifypulse-darwin-arm64", bin: "notifypulse" },
  "win32-x64": { pkg: "@cloudboxsh/notifypulse-win32-x64", bin: "notifypulse.exe" },
};

const key = `${platform}-${arch}`;
const target = PLATFORM_MAP[key];

if (!target) {
  console.error(
    `notifypulse: unsupported platform ${key}.\n` +
      `Supported: ${Object.keys(PLATFORM_MAP).join(", ")}.\n` +
      `Install manually: https://github.com/cloudbox-sh/notifypulse/releases`
  );
  process.exit(1);
}

let pkgDir;
try {
  pkgDir = path.dirname(require.resolve(`${target.pkg}/package.json`));
} catch {
  console.error(
    `notifypulse: platform package ${target.pkg} is not installed.\n` +
      `This usually means your package manager stripped optionalDependencies.\n` +
      `Reinstall without --no-optional, or install the platform package directly:\n` +
      `  npm i ${target.pkg}`
  );
  process.exit(1);
}

const binPath = path.join(pkgDir, "bin", target.bin);
if (!existsSync(binPath)) {
  console.error(`notifypulse: binary missing at ${binPath}. Reinstall @cloudboxsh/notifypulse.`);
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), {
  stdio: "inherit",
  windowsHide: true,
});

if (result.error) {
  console.error(`notifypulse: failed to execute binary: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status ?? 1);
