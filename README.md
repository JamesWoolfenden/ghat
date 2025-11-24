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

Ghat is a tool (GHAT) for updating dependencies in GitHub Actions, GitLab CI/CD, **managing Terraform module and provider versions**, and pre-commit configs. It replaces insecure mutable tags with immutable commit hashes and container image digests, and updates provider versions to their latest stable releases:

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

## Help

```bash
 ghat --help
       _           _
 __ _ | |_   __ _ | |_
/ _` || ' \ / _` ||  _|
\__, ||_||_|\__,_| \__|
|___/
version: v0.1.1
NAME:
   ghat - Update GHA dependencies

USAGE:
   ghat [global options] command [command options] [arguments...]

VERSION:
   v0.1.1

AUTHOR:
   James Woolfenden <jim.wolf@duck.com>

COMMANDS:
   sift, p     updates pre-commit version with  hashes
   shake, k    updates Terraform provider versions to latest stable releases
   stun, t     updates GitLab CI/CD container images with SHA256 digests
   swipe, w    updates Terraform module versions with versioned hashes
   swot, a     updates GHA versions for hashes
   version, v  Outputs the application version
   help, h     Shows a list of commands or help for one command

GLOBAL OPTIONS:
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
