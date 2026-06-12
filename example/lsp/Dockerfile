ARG GO=1.21
FROM golang:${GO} AS build
WORKDIR /src
RUN go build -o /bin/app .

FROM alpine:3.19
COPY --from=build /bin/app /bin/app

FROM gcr.io/distroless/static@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25
