package core

import "testing"

func TestAnalyzeDockerfile(t *testing.T) {
	content := []byte(`ARG GO=1.21
FROM golang:${GO} AS builder
RUN make
FROM gcr.io/distroless/static@sha256:6706c73aae2afaa8201d63cc3dda48753c09bcd6c300762251065c0f7e602b25
FROM scratch
FROM alpine:3.19  # ghat:suppress
`)
	a := AnalyzeDockerfile(content)
	if len(a.Images) != 3 {
		t.Fatalf("expected 3 images (scratch skipped), got %d: %+v", len(a.Images), a.Images)
	}
	if a.Images[0].Tag != "1.21" || a.Images[0].IsDigestPinned || a.Images[0].Line != 2 {
		t.Errorf("images[0] = %+v, want golang:1.21 unpinned at line 2", a.Images[0])
	}
	if !a.Images[1].IsDigestPinned || a.Images[1].Line != 4 {
		t.Errorf("images[1] = %+v, want digest-pinned at line 4", a.Images[1])
	}
	if !a.Images[2].Suppressed || a.Images[2].Line != 6 {
		t.Errorf("images[2] = %+v, want suppressed at line 6", a.Images[2])
	}
}

func TestAnalyzeDockerfile_MultiStageAlias(t *testing.T) {
	content := []byte(`FROM golang:1.22 AS base
RUN go mod download

FROM base AS builder
RUN go build

FROM base
COPY --from=builder /app /app
`)
	a := AnalyzeDockerfile(content)
	if len(a.Images) != 1 {
		t.Fatalf("expected 1 image (stage aliases skipped), got %d: %+v", len(a.Images), a.Images)
	}
	if a.Images[0].Image != "library/golang" || a.Images[0].Tag != "1.22" {
		t.Errorf("images[0] = %+v, want library/golang:1.22", a.Images[0])
	}
}

func TestAnalyzeDockerfile_TrustedRegistries(t *testing.T) {
	content := []byte(`FROM gcr.io/myproject/myimage:v1
FROM us-central1-docker.pkg.dev/myproject/repo/image:latest
FROM nginx:1.25
`)
	trusted := []string{"gcr.io/myproject/", "us-central1-docker.pkg.dev/myproject/"}
	a := AnalyzeDockerfileWithTrusted(content, trusted)
	if len(a.Images) != 3 {
		t.Fatalf("expected 3 images, got %d: %+v", len(a.Images), a.Images)
	}
	if !a.Images[0].Trusted {
		t.Errorf("images[0] should be trusted: %+v", a.Images[0])
	}
	if !a.Images[1].Trusted {
		t.Errorf("images[1] should be trusted: %+v", a.Images[1])
	}
	if a.Images[2].Trusted {
		t.Errorf("images[2] should NOT be trusted: %+v", a.Images[2])
	}
}
