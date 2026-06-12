# Editor integration

`ghat lsp` is a Language Server Protocol server that re-analyses each open
buffer on every keystroke and surfaces findings as native editor diagnostics:

| code | file kinds | meaning |
| --- | --- | --- |
| `ghat-pin` | GHA, pre-commit | `uses:` / `rev:` is not a 40-char commit SHA |
| `ghat-image-pin` | GitLab CI, Dockerfile | image is not pinned to a `sha256:` digest |
| `ghat-permissions` | GHA | no top-level `permissions:` block |
| `ghat-write-all` | GHA | `permissions: write-all` |
| `ghat-dangerous-trigger` | GHA | `pull_request_target` + PR-head checkout, or `github.event.*` in `run:` |
| `ghat-timeout` | GHA, GitLab CI | job has no `timeout-minutes:` / `timeout:` |
| `ghat-secret-env` | GHA | step exposes `${{ secrets.* }}` in `env:` |
| `ghat-concurrency` | GHA | no top-level `concurrency:` block |

Hover a finding for rationale and remediation. Quick-fix code actions:

- **Pin to SHA** — resolves the action/hook tag against the GitHub API and
  rewrites the ref to `<sha> # <tag>`
- **Pin to digest** — resolves the container image against its registry and
  rewrites it to `image:tag@sha256:…` (Dockerfile) or `image@sha256:… # tag`
  (GitLab CI)
- **Insert least-privilege permissions block**
- **Suppress (ghat:suppress)**

The only prerequisite is the `ghat` binary on `PATH`:

```sh
go install github.com/jameswoolfenden/ghat@latest
```

Set `GITHUB_TOKEN` so the *Pin to SHA* and *Pin to digest (ghcr.io)* actions
can authenticate.

## VS Code

Install the **ghat** extension (`make vscode-ext` builds `bin/ghat.vsix`;
install via `Extensions → … → Install from VSIX`).

Settings:

| Key | Default | |
| --- | --- | --- |
| `ghat.path` | `ghat` | binary path |
| `ghat.githubToken` | `""` | PAT for the pin actions (falls back to `$GITHUB_TOKEN`) |

## JetBrains (IntelliJ Ultimate, GoLand, PyCharm Pro, WebStorm, …)

Install the **ghat** plugin (`make jetbrains-ext` builds
`bin/ghat-jetbrains.zip`; install via
`Settings → Plugins → ⚙ → Install Plugin from Disk…`).

Requires a 2024.3+ **commercial** JetBrains IDE — the platform LSP API is not
available in Community Edition. Override the binary path with the `GHAT_BIN`
environment variable.

### Community Edition (LSP4IJ)

Install [LSP4IJ] from the marketplace, then
`Settings → Languages & Frameworks → Language Servers → +` and create a server
with:

| | |
| --- | --- |
| Name | `ghat` |
| Command | `ghat lsp` |
| Mappings → File name patterns | `.github/workflows/*.yml`, `.gitlab-ci.yml`, `.pre-commit-config.yaml`, `Dockerfile*` |

## Neovim

```lua
vim.api.nvim_create_autocmd({ "BufRead", "BufNewFile" }, {
  pattern = {
    "*/.github/workflows/*.yml", "*/.github/workflows/*.yaml",
    ".gitlab-ci.yml", ".pre-commit-config.yaml",
    "Dockerfile", "Dockerfile.*", "Containerfile", "*.dockerfile",
  },
  callback = function()
    vim.lsp.start({
      name = "ghat",
      cmd = { "ghat", "lsp" },
      root_dir = vim.fs.root(0, { ".git" }),
    })
  end,
})
```

## Helix

`~/.config/helix/languages.toml`:

```toml
[language-server.ghat]
command = "ghat"
args = ["lsp"]

[[language]]
name = "yaml"
language-servers = ["yaml-language-server", "ghat"]

[[language]]
name = "dockerfile"
language-servers = ["docker-langserver", "ghat"]
```

(ghat ignores YAML files that aren't workflows / GitLab CI / pre-commit, so a
blanket `yaml` association is harmless.)

## Zed

`~/.config/zed/settings.json`:

```json
{
  "lsp": {
    "ghat": { "binary": { "path": "ghat", "arguments": ["lsp"] } }
  },
  "languages": {
    "YAML":       { "language_servers": ["yaml-language-server", "ghat"] },
    "Dockerfile": { "language_servers": ["dockerfile-language-server", "ghat"] }
  }
}
```

[LSP4IJ]: https://plugins.jetbrains.com/plugin/23257-lsp4ij
