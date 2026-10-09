# holden:ignore:HLD_DF_003 this image is ghat's Docker container action (action.yml
# runs.using: docker). GitHub requires container actions to run as the default
# root user, or they cannot write to GITHUB_WORKSPACE, which ghat edits in place.
FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
# Copy only the Go sources the build needs, not the whole context (.git etc.).
COPY main.go ./
COPY src/ ./src/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/jameswoolfenden/ghat/src/version.Version=${VERSION}" -o /out/ghat .

FROM alpine:3.22@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601
# holden:ignore:HLD_DF_021 the base image is digest-pinned; Alpine only keeps the
# latest build of each package in a release branch, so exact apk pins break the
# build on every security update instead of picking it up.
RUN apk --no-cache add bash git ca-certificates
COPY --from=build /out/ghat /usr/bin/ghat
COPY --chmod=0755 entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

LABEL org.opencontainers.image.source="https://github.com/JamesWoolfenden/ghat"
LABEL org.opencontainers.image.authors="JamesWoolfenden"
