# Learning ghat: a hands-on lesson

This is a guided walk-through of `ghat` for someone who has never used it. By the end you will have pinned a real repository's dependencies to immutable references, audited them for supply-chain risk, and know how to roll the same change out across an entire GitHub org.

For the terse reference version of everything here, see [README.md](README.md).

---

## 1. The problem ghat solves

Almost every CI pipeline you've written trusts a string like `actions/checkout@v4`, `golang:1.21`, or `~> 5.0`. Those strings are **mutable** — the publisher (or anyone who compromises the publisher) can change what they point to *after* you've reviewed them. That's how the 2024 `tj-actions/changed-files` and 2021 Codecov attacks worked: a tag that thousands of repos already trusted was silently repointed to a malicious commit.

`ghat` rewrites those mutable references into **immutable** ones — git SHAs and image digests — and keeps the human-readable tag as a comment so the file stays reviewable:

```yaml
# before
uses: actions/checkout@v4
# after
uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
```

One binary covers GitHub Actions, GitLab CI, Dockerfiles, Kubernetes manifests, Terraform providers & modules, pre-commit hooks, and git submodules.

---

## 2. Setup

### Install

Pick whichever fits your machine:

```shell
# macOS
brew tap jameswoolfenden/homebrew-tap
brew install jameswoolfenden/tap/ghat

# Windows
scoop bucket add iac https://github.com/JamesWoolfenden/scoop.git
scoop install ghat

# anywhere with Go
go install github.com/jameswoolfenden/ghat@latest

# Docker (no install)
docker run --rm -v "$PWD:/repo" jameswoolfenden/ghat sweep -d /repo
```

### Authenticate

ghat talks to the GitHub API a lot. Anonymous access works but you'll hit rate limits within a minute on any non-trivial repo. Export a PAT first:

```shell
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
```

(For GitLab features, `GITLAB_TOKEN` is read the same way.)

Verify the install:

```shell
ghat version
```

---

## 3. Your first pin — GitHub Actions

`cd` into any repo that has a `.github/workflows/` directory.

Always start with `--dry-run` so you can see the diff without writing anything:

```shell
ghat swot -d . --dry-run
```

You'll get a coloured diff for every workflow file. When you're happy:

```shell
ghat swot -d .
git diff   # review what changed
```

### Useful `swot` flags

| Flag | What it does |
| --- | --- |
| `-f <file>` | Pin a single workflow instead of the whole directory |
| `--stable 14` | Ignore releases younger than 14 days — protects you from a freshly-tagged compromise |
| `--pin-only` | Convert the tag you already have to its SHA, **don't** upgrade to latest |
| `--continue-on-error` | Keep going if one action can't be resolved |
| `--no-cache` / `--cache-ttl 1h` | Control the on-disk API cache (default TTL 24h) |

### Tag-mutation warnings

If a workflow was already pinned and ghat sees that the **same tag now resolves to a different SHA**, it prints:

```text
WARN  SUSPICIOUS: actions/foo@v1.2.3 — SHA changed from abc123… to def456… with the same tag.
```

That's the signature of a tag-rewrite attack. Stop and investigate before committing.

---

## 4. One command for the whole repo — `sweep`

Most repos contain more than one kind of dependency. Rather than remembering which sub-command covers which file type, run them all:

```shell
ghat sweep -d . --dry-run
```

`sweep` (alias `all`) runs every pinner ghat has against the directory:

| File found | Pinner run | What changes |
| --- | --- | --- |
| `.github/workflows/*.yml` | `swot` | `uses:` → SHA |
| `.gitlab-ci.yml` | `stun` | `image:` → sha256 digest |
| `.pre-commit-config.yaml` | `sift` | `rev:` → SHA |
| `*.tf` modules | `swipe` | registry source → `git::…?ref=<sha>` |
| `*.tf` providers | `shake` | version constraint → exact latest |
| `Dockerfile*` | `dock` | `FROM` → sha256 digest |
| K8s manifests | `kube` | container `image:` → sha256 digest |
| `.gitmodules` | `sub` | submodule → latest tagged SHA |

`sweep` accepts the union of the underlying flags — `--stable`, `--pin-only`, `--update` (bump Terraform modules to latest), `--token`, etc.

This is the command you put in CI.

---

## 5. The individual pinners

You only need these when you want to target one ecosystem. They all share the same shape: `-d`/`-f` to pick the target, `--dry-run` to preview.

```shell
ghat stun  -d .                # GitLab CI images → digests
ghat sift  -d .                # pre-commit revs → SHAs
ghat swipe -d . --update       # Terraform modules → git SHA (and bump)
ghat shake -d .                # Terraform providers → latest exact version
ghat kube  -d ./k8s            # K8s container images → digests
ghat dock  -d .                # Dockerfile FROM → digests
ghat sub   -d .                # git submodules → latest release SHA
```

> **Mnemonic:** every verb is something you do *to* a hat — swot, stun, swipe, sift, shake, sweep.

---

## 6. Substitutions — retiring abandoned actions

Sometimes pinning isn't enough because the upstream is dead or hostile. ghat can swap a reference for a maintained fork *before* pinning.

Create `~/.ghat.yml` (global) or `.ghat.yml` in the repo root:

```yaml
substitutions:
  - from: old-org/abandoned-action
    to:   your-org/maintained-fork
```

Now every `swot` / `sift` run rewrites `old-org/abandoned-action@anything` to your fork and pins to *its* latest SHA. ghat ships a small built-in list; repo-level config overrides global, which overrides built-in.

---

## 7. Auditing what you depend on — `audit`

Pinning freezes *which* code you run. `audit` tells you whether that code is itself trustworthy.

```shell
ghat audit -d .
```

ghat resolves every dependency it can find (Go modules, GHA actions, pre-commit repos, Terraform modules, npm, PyPI, Cargo, RubyGems) back to its source repo and runs six checks:

| Check | Bucket | Passes when… |
| --- | --- | --- |
| `ci-pinned` | **RISK** | the dep's own workflows pin every `uses:` to a SHA |
| `permissions` | **RISK** | every workflow declares a top-level `permissions:` block |
| `dangerous-trigger` | **RISK** | no `pull_request_target` + PR-head checkout, no `${{ github.event.* }}` in `run:` |
| `signed-pin` | STALE | the SHA you pinned is a verified-signed commit |
| `maintained` | STALE | a release or push in the last 365 days |
| `alive` | STALE | repo exists and isn't archived |

Output looks like:

```text
[RISK ] gha   actions/checkout            3/6
        ✓ signed-pin  ✗ ci-pinned (0/21)  ✗ permissions  ✗ dangerous-trigger  ✓ maintained  ✓ alive
[ok   ] gha   goreleaser/goreleaser-action  6/6

         total    ok  risk stale
  gha       13     1    12     0
```

The process **exits 1** if any dependency lands in `RISK`, so you can wire it into CI as a gate.

Narrow the scope with `--source`:

```shell
ghat audit --source go,gha          # only Go modules + Actions
ghat audit --source go --deep       # include transitive Go deps via `go list -m all`
```

---

## 8. Rolling it out everywhere — `org`

Once one repo is clean, `org` does the same to every repo you own.

```shell
# see what would change across the whole org, touch nothing
ghat org --owner my-org --dry-run

# pin everything and open a PR per repo
ghat org --owner my-org --pr

# enable auto-merge on those PRs where the repo allows it
ghat org --owner my-org --pr --auto-merge

# only specific repos
ghat org --repo my-org/api --repo my-org/infra --pr

# GitLab group (gitlab.com or self-hosted)
ghat org --provider gitlab --owner my-group --pr
ghat org --provider gitlab --base-url https://gitlab.example.com --owner my-group --pr
```

Each repo is shallow-cloned to a temp dir, `sweep` is run, and (with `--pr`) the result is pushed to `ghat/pin-dependencies` and a PR/MR opened. Re-running is **idempotent** — existing branches are force-pushed and existing PRs refreshed, never duplicated.

For very large estates use `--offset N --limit M` to shard the run, and `--rate-threshold 200` (default) makes ghat pause before it exhausts your GitHub rate limit.

---

## 9. Keeping it pinned — automation

### As a GitHub Action

```yaml
name: ghat
on:
  schedule:
    - cron: 0 4 * * 1   # Monday 04:00
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  pin:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: JamesWoolfenden/ghat@v0   # ghat will rewrite this to a SHA on first run
        with:
          directory: .
      - uses: peter-evans/create-pull-request@v8
        with:
          commit-message: "chore: pin dependencies to immutable refs"
          title: "chore: pin dependencies to immutable refs"
          branch: ghat/pin
```

Or fail any PR that introduces an unpinned ref:

```yaml
- uses: JamesWoolfenden/ghat@v0
  with:
    verb: swot
    directory: .github/workflows
    dryrun: true
```

### As a pre-commit hook

```yaml
- repo: https://github.com/JamesWoolfenden/ghat
  rev: v0.2.0
  hooks:
    - id: ghat-go
      entry: ghat --quiet swot -d .
      pass_filenames: false
      always_run: true
```

The global `--quiet` flag suppresses the banner so hook output stays clean.

---

## 10. Housekeeping — the cache

ghat caches GitHub/registry responses on disk for 24h so repeat runs are fast and rate-limit-friendly.

```shell
ghat cache stats   # how big, how many entries
ghat cache clean   # drop expired entries
ghat cache clear   # drop everything
```

Per-run overrides: `--no-cache`, `--cache-ttl 1h`, `--clear-cache`.

---

## Cheat sheet

```text
ghat sweep -d . --dry-run     # preview every pin in this repo
ghat sweep -d .               # do it
ghat audit -d .               # score deps, exit 1 on RISK
ghat org --owner X --pr       # PR pins across an entire org
ghat swot  -d . --stable 14   # GHA only, ignore releases <14d old
ghat <verb> --help            # full flag list for any verb
```
