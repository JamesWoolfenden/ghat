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

Ghat will use your Github creds, if available, from your environment using the environmental variables GITHUB_TOKEN or GITHUB_API, but it can also drop back to anonymous access, the drawback is that this is severely rate limited by gitHub.

## Table of Contents

<!--toc:start-->
- [ghat](#ghat)
  - [Table of Contents](#table-of-contents)
  - [Install](#install)
    - [MacOS](#macos)
    - [Windows](#windows)
    - [Docker](#docker)
  - [Usage](#usage)

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
docker run --tty --volume /local/path/to/tf:/tf jameswoolfenden/ghat scan -d /tf
```

<https://hub.docker.com/repository/docker/jameswoolfenden/ghat>

## Usage

To authenticate the GitHub Api you will need to set you GitHub Personal Access Token as the environment variable
*GITHUB_API*

## Help

```bash
./ghat -h
NAME:
   ghat - Update GHA dependencies

USAGE:
   ghat [global options] command [command options] [arguments...]

VERSION:
   9.9.9

AUTHOR:
   James Woolfenden <jim.wolf@duck.com>

COMMANDS:
   swot, a     updates GHA in a directory
   version, v  Outputs the application version
   help, h     Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version

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
