#!/usr/bin/env node
// Builds the 5 per-platform npm packages from GoReleaser output.
//
// Expects:
//   dist/                          — GoReleaser's output (checked into CI artefacts)
//   VERSION env var                — version number without leading "v"
// Produces:
//   npm/dist/platforms/<pkg>/bin/  — the prebuilt binary
//   npm/dist/platforms/<pkg>/package.json
//   npm/dist/platforms/<pkg>/README.md
//
// Run: node npm/scripts/build-platform-packages.js

"use strict";

const { execFileSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const VERSION = process.env.VERSION;
if (!VERSION) {
  console.error("build-platform-packages: VERSION env var is required (no leading 'v').");
  process.exit(1);
}

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const DIST_DIR = path.join(REPO_ROOT, "dist");
const OUT_DIR = path.join(REPO_ROOT, "npm", "dist", "platforms");

// Mirrors the GoReleaser `name_template` in .goreleaser.yaml.
// GoReleaser emits archives named:
//   notifypulse_<version>_<os>_<x86_64|arm64>.<tar.gz|zip>
const TARGETS = [
  {
    pkg: "@cloudboxsh/notifypulse-linux-x64",
    dir: "linux-x64",
    archive: `notifypulse_${VERSION}_linux_x86_64.tar.gz`,
    binary: "notifypulse",
    os: ["linux"],
    cpu: ["x64"],
  },
  {
    pkg: "@cloudboxsh/notifypulse-linux-arm64",
    dir: "linux-arm64",
    archive: `notifypulse_${VERSION}_linux_arm64.tar.gz`,
    binary: "notifypulse",
    os: ["linux"],
    cpu: ["arm64"],
  },
  {
    pkg: "@cloudboxsh/notifypulse-darwin-x64",
    dir: "darwin-x64",
    archive: `notifypulse_${VERSION}_darwin_x86_64.tar.gz`,
    binary: "notifypulse",
    os: ["darwin"],
    cpu: ["x64"],
  },
  {
    pkg: "@cloudboxsh/notifypulse-darwin-arm64",
    dir: "darwin-arm64",
    archive: `notifypulse_${VERSION}_darwin_arm64.tar.gz`,
    binary: "notifypulse",
    os: ["darwin"],
    cpu: ["arm64"],
  },
  {
    pkg: "@cloudboxsh/notifypulse-win32-x64",
    dir: "win32-x64",
    archive: `notifypulse_${VERSION}_windows_x86_64.zip`,
    binary: "notifypulse.exe",
    os: ["win32"],
    cpu: ["x64"],
  },
];

fs.mkdirSync(OUT_DIR, { recursive: true });

for (const t of TARGETS) {
  const archivePath = path.join(DIST_DIR, t.archive);
  if (!fs.existsSync(archivePath)) {
    console.error(`build-platform-packages: archive missing: ${archivePath}`);
    process.exit(1);
  }

  const pkgDir = path.join(OUT_DIR, t.dir);
  const binDir = path.join(pkgDir, "bin");
  fs.rmSync(pkgDir, { recursive: true, force: true });
  fs.mkdirSync(binDir, { recursive: true });

  // Extract the binary out of the archive into bin/.
  if (t.archive.endsWith(".tar.gz")) {
    execFileSync("tar", ["-xzf", archivePath, "-C", binDir, t.binary], { stdio: "inherit" });
  } else if (t.archive.endsWith(".zip")) {
    // -j flatens paths; -o overwrites.
    execFileSync("unzip", ["-jo", archivePath, t.binary, "-d", binDir], { stdio: "inherit" });
  } else {
    console.error(`build-platform-packages: unknown archive format: ${t.archive}`);
    process.exit(1);
  }

  if (process.platform !== "win32") {
    fs.chmodSync(path.join(binDir, t.binary), 0o755);
  }

  const pkgJson = {
    name: t.pkg,
    version: VERSION,
    description: `Prebuilt ${t.dir} binary for @cloudboxsh/notifypulse.`,
    repository: {
      type: "git",
      url: "git+https://github.com/cloudbox-sh/notifypulse.git",
    },
    license: "MIT",
    preferUnplugged: true,
    os: t.os,
    cpu: t.cpu,
    files: ["bin/"],
  };

  fs.writeFileSync(
    path.join(pkgDir, "package.json"),
    JSON.stringify(pkgJson, null, 2) + "\n"
  );

  fs.writeFileSync(
    path.join(pkgDir, "README.md"),
    `# ${t.pkg}\n\n` +
      `Prebuilt \`notifypulse\` binary for ${t.dir}.\n\n` +
      `You probably want the meta package instead:\n\n` +
      `    npm i -g @cloudboxsh/notifypulse\n`
  );

  console.log(`built ${t.pkg} @ ${VERSION}`);
}

console.log(`\n${TARGETS.length} platform package(s) ready in ${OUT_DIR}`);
