# ghat

![alt text](ghat.png "ghat")

[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://GitHub.com/jameswoolfenden/ghat/graphs/commit-activity)
[![Build Status](https://github.com/JamesWoolfenden/ghat/workflows/CI/badge.svg?branch=master)](https://github.com/JamesWoolfenden/ghat)
[![Latest Release](https://img.shields.io/github/release/JamesWoolfenden/ghat.svg)](https://github.com/JamesWoolfenden/ghat/releases/latest)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/JamesWoolfenden/ghat.svg?label=latest)](https://github.com/JamesWoolfenden/ghat/releases/latest)
![Terraform Version](https://img.shields.io/badge/tf-%3E%3D0.14.0-blue.svg)
[![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit&logoColor=white)](https://github.com/pre-commit/pre-commit)
[![checkov](https://img.shields.io/badge/checkov-verified-brightgreen)](https://www.checkov.io/)
[![Github All Releases](https://img.shields.io/github/downloads/jameswoolfenden/ghat/total.svg)](https://github.com/JamesWoolfenden/ghat/releases)
[![codecov](https://codecov.io/gh/JamesWoolfenden/ghat/graph/badge.svg?token=P9V791WMRE)](https://codecov.io/gh/JamesWoolfenden/ghat)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/JamesWoolfenden/ghat/badge)](https://securityscorecards.dev/viewer/?uri=github.com/JamesWoolfenden/ghat)

Ghat is a tool (GHAT) for updating dependencies in GitHub Actions, GitLab CI/CD, Kubernetes manifests, **managing Terraform module and provider versions**, and pre-commit configs. It replaces insecure mutable tags with immutable commit hashes and container image digests, and updates provider versions to their latest stable releases:

```yml
   ## sets up go based on the version
      - name: Install Go
        uses: actions/setup-go@v4.0.1
        with:
          go-version: ${{ matrix.go-version }}

      ## checks out our code locally, so we can work with the files
      - name: Checkout code
        uses: actions/checkout@v3.5.3
```

Becomes

```yml
      ## sets up go based on the version
      - name: Install Go
        uses: actions/setup-go@fac708d6674e30b6ba41289acaab6d4b75aa0753 # v4.0.1
        with:
          go-version: ${{ matrix.go-version }}

      ## checks out our code locally, so we can work with the files
      - name: Checkout code
        uses: actions/checkout@c85c95e3d7251135ab7dc9ce3241c5835cc595a9 # v3.5.3
```

Ghat will use your GitHub credentials, if available, from your environment using the environmental variables GITHUB_TOKEN or GITHUB_API, but it can also drop back to anonymous access, the drawback is that this is severely rate limited by gitHub.

Ghat also manages GitLab CI/CD container images by replacing mutable tags with immutable SHA256 digests:

```yaml
build-job:
  stage: build
  image: golang:1.21
  script:
    - go build
```

Becomes:

```yaml
build-job:
  stage: build
  image: golang@sha256:4746d26432a9117a5f58e95cb9f954ddf0de128e9d5816886514199316e4a2fb # 1.21
  script:
    - go build
```

It manages Terraform provider versions by querying the Terraform Registry and updating to the latest stable versions:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
```

Becomes:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.22.1"
    }
  }
}
```

And it manages Terraform modules, to give you the most secure reference, so:

```terraform
module "ip" {
  source      = "JamesWoolfenden/ip/http"
  version     = "0.3.12"
  permissions = "pike"
}
```

Becomes:

```terraform
module "ip" {
  source      = "git::https://github.com/JamesWoolfenden/terraform-http-ip.git?ref=a6cf071d14365133f48ed161812c14b00ad3c692"
  permissions = "pike"
}

```

> **New to ghat?** Start with the [hands-on tutorial](TUTORIAL.md) — it walks you from first pin to org-wide rollout.

## Table of Contents

<!--toc:start-->
- [ghat](#ghat)
  - [Table of Contents](#table-of-contents)
  - [Install](#install)
    - [MacOS](#macos)
    - [Windows](#windows)
    - [Docker](#docker)
  - [Usage](#usage)
    - [swot](#swot)
      - [directory](#directory-scan)
      - [file](#file-scan)
      - [stable](#stable-releases)
    - [stun](#stun)
      - [directory scan](#directory-scan-1)
      - [dry-run](#dry-run)
    - [shake](#shake)
      - [directory scan](#directory-scan-2)
      - [file scan](#file-scan-1)
    - [swipe](#swipe)
    - [sift](#sift)
    - [kube](#kube)
    - [sweep](#sweep)
    - [audit](#audit)
    - [org](#org)
    - [pre-commit](#pre-commit)

<!--toc:end-->

## Install

Download the latest binary here:

<https://github.com/JamesWoolfenden/ghat/releases>

Install from code:

- Clone repo
- Run `go install`

Install remotely:

```shell
go install  github.com/jameswoolfenden/ghat@latest
```

### MacOS

```shell
brew tap jameswoolfenden/homebrew-tap
brew install jameswoolfenden/tap/ghat
```

### Windows

I'm now using Scoop to distribute releases, it's much quicker to update and easier to manage than previous methods,
you can install scoop from <https://scoop.sh/>.

Add my scoop bucket:

```shell
scoop bucket add iac https://github.com/JamesWoolfenden/scoop.git
```

Then you can install a tool:

```bash
scoop install ghat
```

### Docker

```shell
docker pull jameswoolfenden/ghat
docker run --tty --volume /local/path/to/repo:/repo jameswoolfenden/ghat swot -d /repo
```

<https://hub.docker.com/repository/docker/jameswoolfenden/ghat>

### GitHub Action

Pin everything in your repo on a schedule and open a PR with the changes:

```yaml
name: ghat
on:
  schedule:
    - cron: 0 4 * * 1
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  pin:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: JamesWoolfenden/ghat@v0  # ghat will rewrite this to a SHA
        with:
          directory: .
      - uses: peter-evans/create-pull-request@v8
        with:
          commit-message: "chore: pin dependencies to immutable refs"
          title: "chore: pin dependencies to immutable refs"
          branch: ghat/pin
```

Or fail a PR that introduces unpinned refs:

```yaml
- uses: JamesWoolfenden/ghat@v0
  with:
    verb: swot
    directory: .github/workflows
    dryrun: true
```

Inputs: `verb` (default `sweep`), `directory` (default `.`), `file`, `dryrun`, `github_token` (default `${{ github.token }}`).

## Usage

To authenticate the GitHub Api you should set up your GitHub Personal Access Token as the environment variable
*GITHUB_API* or *GITHUB_TOKEN*, it will fall back to using anonymous if you don't but RATE LIMITS.

### swot

#### Directory scan

This will look for the .github/workflow folder and update all the files it finds there, and display a diff of the changes made to each file:

```bash
$ghat swot -d .
```

#### File scan

```bash
$ghat swot -f .\.github\workflows\ci.yml
```

#### Stable releases

If you're concerned that the very latest release might be too fresh, and would rather have the latest from 2 weeks ago?
I got you covered:

```bash
$ghat swot -d . --stable 14
```

#### Tag mutation detection

When `swot` processes a workflow file that already has a pinned action (`action@sha # vX.Y.Z`), it checks whether the SHA GitHub now resolves for that same tag matches what was previously pinned. If the tag name is unchanged but the SHA has changed, ghat emits a warning:

```text
WARN  SUSPICIOUS: actions/checkout@v4.2.2 — SHA changed from abc123... to def456... with the same tag.
      The tag may have been moved to a different commit. Verify this is intentional before accepting.
```

This is a sign that a repository maintainer (or attacker) has rewritten a published tag to point to a different commit — the pattern behind supply chain attacks like those reported via Dependabot. The warning includes the new commit's signature status (`signed, verified` / `signed, unverified — <reason>` / `UNSIGNED`) to help triage. **Do not accept the update without reviewing the new commit.**

## Substitutions

Sometimes an action or pre-commit hook you depend on is abandoned, taken over, or superseded by a fork. Substitutions let ghat swap the old reference for a trusted replacement before pinning, so every repo that references the old name gets silently migrated.

### Built-in defaults

ghat ships with a set of known substitutions embedded in the binary (`src/core/substitutions.yml`):

```yaml
substitutions:
  - from: iamnotaturtle/auto-gofmt
    to: JamesWoolfenden/auto-gofmt
```

These are applied automatically — no configuration required.

### User-defined substitutions

Add your own rules in `~/.ghat.yml`. They are merged with (and can override) the built-in defaults:

```yaml
substitutions:
  - from: old-org/abandoned-action
    to: your-org/maintained-fork
```

### Per-repo substitutions

A `.ghat.yml` in the root of the repo being processed is also loaded and merged last, so it takes precedence over both the built-in defaults and your global config:

```yaml
substitutions:
  - from: company/legacy-action
    to: company/new-action
```

### What gets substituted

- `uses:` lines in GitHub Actions workflows (the action owner/repo is swapped and re-pinned to the fork's latest SHA)
- `repo:` lines in `.pre-commit-config.yaml` (both the URL and the `rev:` are rewritten)

### stun

Stun updates GitLab CI/CD container image references to use immutable SHA256 digests instead of mutable tags. This prevents supply chain attacks through image tampering and ensures build reproducibility.

#### Directory scan

This will look for `.gitlab-ci.yml` in the directory and update all container image references:

```bash
$ghat stun -d .
```

#### Dry-run

Preview changes without modifying the file:

```bash
$ghat stun -d . --dry-run
```

**Features:**

- Supports both simple (`image: golang:1.21`) and complex (`image: { name: golang:1.21 }`) image formats
- Works with Docker Hub, GitHub Container Registry (ghcr.io), and custom OCI registries
- Automatically skips GitLab CI variables (e.g., `$CI_REGISTRY_IMAGE`)
- Preserves original tag as a comment for readability
- Shows colored diff of changes

**Example output:**

```yaml
# Before
image: node:18-alpine

# After
image: node@sha256:8d6421d663b4c28fd3ebc498332f249011d118945588d0a35cb9bc4b8ca09d9e # 18-alpine
```

### shake

Shake updates Terraform provider versions to their latest stable releases by querying the Terraform Registry API. It replaces version constraints with specific version numbers.

#### Directory scan

Scan all Terraform files in a directory for provider updates:

```bash
$ghat shake -d .
```

#### File scan

Update providers in a specific file:

```bash
$ghat shake -f providers.tf
```

**Features:**

- Queries the official Terraform Registry API for latest versions
- Replaces version constraints (`~>`, `>=`, etc.) with specific versions
- Supports all Terraform Registry providers
- Skips pre-release versions by default
- Shows colored diff of changes
- `--dryrun` flag for preview
- `--continue-on-error` flag to process all files even if some fail

**Example:**

```hcl
# Before
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0"
    }
  }
}

# After
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "6.22.1"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.7.2"
    }
  }
}
```

### Swipe

Updates Terraform modules to use secure module references, and displays a file diff:

```bash
 ghat swipe -f .\registry\module.git.tf -update
        _           _
 __ _ | |_   __ _ | |_
/ _` || ' \ / _` ||  _|
\__, ||_||_|\__,_| \__|
|___/
version: 9.9.9
1:42PM INF module source is git::https://github.com/terraform-aws-modules/terraform-aws-memory-db.git?depth=1 of type shallow and cannot be updated
module "ip" {
  source      = "git::https://github.com/JamesWoolfenden/ip/terraform-http"
  v-ip.git?rersion     f= "aca5d0.4513.1698f2f564913cfcc3534780794c800"
  permissions = "pike"
}
```

The update flag can be used to update the reference, the default behaviour is just to change the reference to a git bashed hash.

### sift

Sift updates pre-commit configs with the latest hooks using hashes.
Commands are similar, but only the directory is needed:

```shell
ghat sift -d .
```

The flag dryrun is also supported. Example outcome display:

```yaml
    - hooks:
        - id: forbid-tabs
          exclude: binary|\.bin$|rego|\.rego$|go|\.go$
          exclude_types:
            - python
            - javascript
            - dtd
            - markdown
            - makefile
            - xml
      repo: https://github.com/Lucas-C/pre-commit-hooks
      rev: 762c66ea96843b54b936fc680162ea67f85ec2d7
```

### kube

Kube pins container image references in Kubernetes manifests to immutable SHA256 digests, preventing supply chain attacks through mutable image tags. It supports Deployment, StatefulSet, DaemonSet, Job, CronJob, ReplicaSet, and Pod resources, including multi-document YAML files.

#### Directory scan

```bash
ghat kube -d ./k8s
```

#### File scan

```bash
ghat kube -f deployment.yaml
```

#### Dry run

```bash
ghat kube -d . --dryrun
```

A manifest like:

```yaml
spec:
  template:
    spec:
      initContainers:
        - name: init
          image: busybox:1.36
      containers:
        - name: app
          image: nginx:1.25
```

Becomes:

```yaml
spec:
  template:
    spec:
      initContainers:
        - name: init
          image: busybox@sha256:37f7b378a29ceb4c551b1b5582e27747b855bbfaa73fa11914fe0df028dc581f # 1.36
      containers:
        - name: app
          image: nginx@sha256:a484819eb60211f5299034ac80f6a681b06f89e65866ce91f356ed7c72af059c # 1.25
```

Variable references such as `$(IMAGE_TAG)` are skipped automatically.

### sweep

Runs every pinner (swot, stun, sift, swipe, shake, kube, dock) against a directory in one pass.

```shell
ghat sweep -d .
```

Useful in CI when you don't want to enumerate which file types a repo contains.

### audit

Scores each of your dependencies as a supply-chain risk. Reads go.mod, `.github/workflows/`, `.pre-commit-config.yaml`, and Terraform module sources, resolves each to its GitHub repo, then runs six checks against that repo and buckets it as `ok`, `STALE`, or `RISK`. Exits 1 if any `RISK` deps are found.

```shell
ghat audit -d .
ghat audit -d . --source go,gha
ghat audit -d . --source go --deep
```

`--source` narrows to one or more of `go`, `gha`, `pre-commit`, `terraform`,
`npm`, `pypi`, `cargo`, `gem` (default: all that have a manifest present).
`--deep` walks transitive Go modules via `go list -m all`.

| source | manifest read | repo resolved via |
| --- | --- | --- |
| `go` | `go.mod` | go-import meta tag |
| `gha` | `.github/workflows/*.yml` | `uses:` owner/repo |
| `pre-commit` | `.pre-commit-config.yaml` | `repo:` URL |
| `terraform` | `*.tf` | module `source` |
| `npm` | `package.json` | registry.npmjs.org |
| `pypi` | `requirements*.txt`, `pyproject.toml` | pypi.org |
| `cargo` | `Cargo.toml` | crates.io |
| `gem` | `Gemfile` | rubygems.org |

Checks (✓ pass / ✗ fail / - n/a):

| check | severity | what it means |
| --- | --- | --- |
| `ci-pinned` | RISK | the dep's own workflows pin every `uses:` to a SHA |
| `permissions` | RISK | every workflow declares a top-level `permissions:` block (no default write-all) |
| `dangerous-trigger` | RISK | no `pull_request_target` + PR-head checkout, no `${{ github.event.* }}` in `run:` |
| `signed-pin` | STALE | the SHA you pinned is a signed/verified commit — catches account takeover, not malicious maintainers |
| `maintained` | STALE | a release or push in the last 365 days |
| `alive` | STALE | repo exists and is not archived/disabled |

Sample output:

```text
[RISK ] gha        actions/checkout                  actions/checkout       3/6
        ✓ signed-pin  ✗ ci-pinned (0/21)  ✗ permissions (7/7 default write-all)  ✗ dangerous-trigger (update-main-version.yml: github.event in run:)  ✓ maintained (115d)  ✓ alive
          codeql-analysis.yml: github/codeql-action/init@v3
          ... and 20 more
[ok   ] gha        goreleaser/goreleaser-action      goreleaser/goreleaser-action  6/6

             total    ok  risk stale
  gha           13     1    12     0
```

#### Fixing `signed-pin` on your own repos

If `✗ signed-pin` flags a repo *you* own, turn on SSH commit signing once
(git ≥ 2.34) and every future commit will pass:

```shell
git config --global gpg.format ssh
git config --global user.signingkey ~/.ssh/id_ed25519.pub   # or id_rsa.pub
git config --global commit.gpgsign true
git config --global tag.gpgsign true
```

Then register the same public key as a **signing** key (separate from auth):

- GitHub: Settings → SSH and GPG keys → New SSH key → Key type: **Signing Key**
- GitLab: Preferences → SSH Keys → Usage type: **Authentication & Signing**

Third-party deps failing `signed-pin` aren't yours to fix — that's why the
check is STALE, not RISK.

### org

Runs `sweep` against every non-fork repo owned by a user, organisation, or GitLab group, and optionally opens a PR/MR with the pinning changes. Use this to roll out SHA-pinning across an entire estate in one shot.

```shell
# dry-run across every repo the token owns
ghat org --dry-run

# pin and open PRs for every repo in an org
ghat org --owner my-org --pr

# target specific repos only
ghat org --repo my-org/app --repo my-org/infra --pr --auto-merge

# GitLab (gitlab.com or self-hosted)
ghat org --provider gitlab --owner my-group --pr
ghat org --provider gitlab --base-url https://gitlab.example.com --owner my-group --pr
```

Each repo is shallow-cloned to a temp dir, swept, and (with `--pr`) pushed to `--branch` (default `ghat/pin-dependencies`) before a PR/MR is opened. Re-running is idempotent: if the branch and PR already exist they're force-pushed and refreshed rather than duplicated. `--auto-merge` enables squash-auto-merge on each PR where the repo allows it.

`--token` falls back to `$GITHUB_TOKEN` / `$GITLAB_TOKEN`. The token needs `repo` scope on GitHub, or `api` + `write_repository` on GitLab. `--offset`/`--limit` let you shard a large org across multiple runs, and `--rate-threshold` pauses before exhausting the GitHub rate limit.

The summary also reports **gaps**: version-pinned installs ghat doesn't yet rewrite (e.g. `go install …@v1.2.3`, `pip install foo==1.0`, `curl …/releases/download/…`) so you can see what's left to lock down by hand.

## Help

```bash
 ghat --help
       _           _
 __ _ | |_   __ _ | |_
/ _` || ' \ / _` ||  _|
\__, ||_||_|\__,_| \__|
|___/
version: v0.1.26
NAME:
   ghat - Update GHA dependencies

USAGE:
   ghat [global options] command [command options]

VERSION:
   v0.1.26

AUTHOR:
   James Woolfenden <jim.wolf@duck.com>

COMMANDS:
   all, sweep  runs every pinner (GHA, GitLab, pre-commit, Terraform, Kubernetes, Dockerfiles) against a directory
   audit, sc   scores your dependencies (go.mod, GHA uses:, pre-commit, Terraform, npm, PyPI, Cargo, RubyGems) on supply-chain hygiene
   cache       Manage API response cache
   dock, df    pins Dockerfile FROM images to SHA digests
   kube, k8s   pins container images in Kubernetes manifests to SHA digests
   org         run ghat all across every non-fork repo for a GitHub/GitLab user, org or group
   shake, k    updates Terraform provider versions to latest
   sift, p     updates pre-commit version with hashes
   stun, t     updates Gitlab versions for hashes
   sub, m      updates git submodule pins to latest tagged release SHA
   swipe, w    updates Terraform module versions with versioned hashes
   swot, a     updates GHA versions for hashes
   version, v  Outputs the application version
   help, h     Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --quiet        suppress banner and log output (useful in pre-commit hooks) (default: false)
   --help, -h     show help
   --version, -v  print the version

COPYRIGHT:
   James Woolfenden
```

### pre-commit

I've added a number of pre-commit hooks to this repo that will update your build configs,
update .pre-commit-config.yaml

```yaml
  - repo: https://github.com/JamesWoolfenden/ghat/actions
    rev: v0.0.10
    hooks:
      - id: ghat-go
        name: ghat
        description: upgrade action dependencies
        language: golang
        entry: ghat swot -d .
        pass_filenames: false
        always_run: true
        types: [ yaml ]

```

## Building

```shell
go build
```

or

```Make
Make build
```

## Extending

Log an issue, a pr or an email to jim.wolf @ duck.com.
