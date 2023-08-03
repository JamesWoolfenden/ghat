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

Ghat is a tool  (GHAT) for updating dependencies in a GHA - GitHub Action. It replaces insecure mutable tags with immutable commit hashes as well as using the latest released version:

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

Ghat will use your GitHub creds, if available, from your environment using the environmental variables GITHUB_TOKEN or GITHUB_API, but it can also drop back to anonymous access, the drawback is that this is severely rate limited by gitHub.

## Table of Contents

<!--toc:start-->
- [ghat](#ghat)
  - [Table of Contents](#table-of-contents)
  - [Install](#install)
    - [MacOS](#macos)
    - [Windows](#windows)
    - [Docker](#docker)
  - [Usage](#usage)
    - [directory](#directory-scan)
    - [file](#file-scan)
    - [stable](#stable-releases)
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

### Directory scan

This will look for the .github/workflow folder and update all the files it finds there, and display a diff of the changes made to each file:

```bash
$ghat swot -d .
```

### File scan

```bash
$ghat swot -f .\.github\workflows\ci.yml
```

### Stable releases

If you're concerned that the very latest release might be too fresh, and would rather have the latest from 2 weeks ago?
I got you covered:

```bash
$ghat swot -d . --stable 14
```

## Help

```bash
 ghat swot -h
NAME:
   ghat swot - updates GHA in a directory

USAGE:
   ghat swot

OPTIONS:
   authentication

   --token value, -t value  Github PAT token [$GITHUB_TOKEN, $GITHUB_API]

   delay

   --stable value, -s value  days to wait for stabilisation of release (default: 0)

   files

   --directory value, -d value  Destination to update GHAs (default: ".")
   --file value, -f value       GHA file to parse

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

```go
go build
```

or

```Make
Make build
```

## Extending

Log an issue, a pr or send an email to jim.wolf @ duck.com.
