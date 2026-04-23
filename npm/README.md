# @cloudboxsh/notifypulse (npm distribution)

Source layout for the npm distribution of the Notifypulse CLI.

Users install:

```sh
npx @cloudboxsh/notifypulse --help
npm i -g @cloudboxsh/notifypulse
```

## How it works (`optionalDependencies` pattern)

At release time we publish **N+1** packages, all at the same version:

| Package | Contents |
|---|---|
| `@cloudboxsh/notifypulse` | Meta package. Node shim + `optionalDependencies` listing every platform package. |
| `@cloudboxsh/notifypulse-linux-x64` | Linux x86_64 prebuilt binary. |
| `@cloudboxsh/notifypulse-linux-arm64` | Linux ARM64 prebuilt binary. |
| `@cloudboxsh/notifypulse-darwin-x64` | macOS Intel prebuilt binary. |
| `@cloudboxsh/notifypulse-darwin-arm64` | macOS Apple Silicon prebuilt binary. |
| `@cloudboxsh/notifypulse-win32-x64` | Windows x86_64 prebuilt binary. |

Each platform package declares `"os"` and `"cpu"` so npm/pnpm/yarn only downloads the one matching the host. The meta package's `bin/notifypulse.js` shim resolves the correct platform package at runtime and execs the binary.

Same industry pattern as esbuild, biome, turbo, swc, bun, @sentry/cli.

## Build flow (done by CI, not locally)

The `.github/workflows/release.yml` `npm` job runs after GoReleaser produces `dist/`. It invokes:

1. `npm/scripts/build-platform-packages.js` — extracts each GoReleaser tarball into `npm/dist/platforms/<name>/bin/` and writes a tailored `package.json`.
2. `npm publish` over each platform package.
3. `npm/scripts/build-meta-package.js` — copies `bin/notifypulse.js` + writes a meta `package.json` with `optionalDependencies` pinned to the same version.
4. `npm publish` the meta.

Nothing in this directory is published directly — only the generated contents of `npm/dist/` are.
