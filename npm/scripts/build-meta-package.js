#!/usr/bin/env node
// Builds the meta @cloudboxsh/notifypulse npm package.
//
// Expects:
//   VERSION env var — version number without leading "v"
// Produces:
//   npm/dist/meta/           — meta package directory ready to `npm publish`
//   ├── package.json
//   ├── README.md
//   └── bin/notifypulse.js
//
// Run: node npm/scripts/build-meta-package.js

"use strict";

const fs = require("node:fs");
const path = require("node:path");

const VERSION = process.env.VERSION;
if (!VERSION) {
  console.error("build-meta-package: VERSION env var is required (no leading 'v').");
  process.exit(1);
}

const REPO_ROOT = path.resolve(__dirname, "..", "..");
const NPM_DIR = path.join(REPO_ROOT, "npm");
const OUT_DIR = path.join(NPM_DIR, "dist", "meta");

const PLATFORM_PACKAGES = [
  "@cloudboxsh/notifypulse-linux-x64",
  "@cloudboxsh/notifypulse-linux-arm64",
  "@cloudboxsh/notifypulse-darwin-x64",
  "@cloudboxsh/notifypulse-darwin-arm64",
  "@cloudboxsh/notifypulse-win32-x64",
];

const optionalDependencies = Object.fromEntries(
  PLATFORM_PACKAGES.map((p) => [p, VERSION])
);

fs.rmSync(OUT_DIR, { recursive: true, force: true });
fs.mkdirSync(path.join(OUT_DIR, "bin"), { recursive: true });

const pkgJson = {
  name: "@cloudboxsh/notifypulse",
  version: VERSION,
  description: "Notifypulse CLI — one-API notification relay from the terminal.",
  keywords: [
    "notifypulse",
    "cloudbox",
    "notifications",
    "telegram",
    "discord",
    "slack",
    "webhook",
    "cli",
  ],
  homepage: "https://notifypulse.cloudbox.sh",
  bugs: "https://github.com/cloudbox-sh/notifypulse/issues",
  repository: {
    type: "git",
    url: "git+https://github.com/cloudbox-sh/notifypulse.git",
  },
  license: "MIT",
  bin: {
    notifypulse: "bin/notifypulse.js",
  },
  files: ["bin/"],
  engines: {
    node: ">=18",
  },
  optionalDependencies,
};

fs.writeFileSync(
  path.join(OUT_DIR, "package.json"),
  JSON.stringify(pkgJson, null, 2) + "\n"
);

fs.copyFileSync(
  path.join(NPM_DIR, "bin", "notifypulse.js"),
  path.join(OUT_DIR, "bin", "notifypulse.js")
);

// Ship a user-facing README instead of the internal build notes.
const rootReadme = path.join(REPO_ROOT, "README.md");
fs.copyFileSync(rootReadme, path.join(OUT_DIR, "README.md"));

console.log(`built @cloudboxsh/notifypulse @ ${VERSION} in ${OUT_DIR}`);
