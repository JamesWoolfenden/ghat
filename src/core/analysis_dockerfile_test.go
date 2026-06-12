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
