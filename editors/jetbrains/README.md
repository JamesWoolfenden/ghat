# ghat — JetBrains plugin

Thin LSP client that launches `ghat lsp` for GitHub Actions workflows,
`.gitlab-ci.yml`, `.pre-commit-config.yaml` and Dockerfiles using the IntelliJ
Platform LSP API. Works in IntelliJ IDEA Ultimate, GoLand, PyCharm
Professional, WebStorm, and other commercial JetBrains IDEs (2024.3+). The
platform LSP API is not available in Community Edition — see
[`docs/editor-integration.md`](../../docs/editor-integration.md) for the LSP4IJ
alternative.

## Build

```sh
./gradlew buildPlugin
```

Produces `build/distributions/ghat-jetbrains-<version>.zip`. From the repo
root: `make jetbrains-ext`.

Behind a TLS-intercepting corporate proxy the wrapper distribution download
often fails; use a system Gradle instead: `make jetbrains-ext GRADLE=gradle`.
The daemon JDK's `cacerts` must trust the corporate root CA.

## Install

`Settings → Plugins → ⚙ → Install Plugin from Disk…` and pick the zip.

## Configuration

The plugin runs `ghat lsp` from `PATH`. Override with the `GHAT_BIN`
environment variable if the binary lives elsewhere.
