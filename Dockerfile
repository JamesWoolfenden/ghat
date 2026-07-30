FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/jameswoolfenden/ghat/src/version.Version=${VERSION}" -o /out/ghat .

FROM alpine:3.22@sha256:310c62b5e7ca5b08167e4384c68db0fd2905dd9c7493756d356e893909057601
RUN apk --no-cache add bash git ca-certificates
COPY --from=build /out/ghat /usr/bin/ghat
COPY --chmod=0755 entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]

LABEL org.opencontainers.image.source="https://github.com/JamesWoolfenden/ghat"
LABEL org.opencontainers.image.authors="JamesWoolfenden"
