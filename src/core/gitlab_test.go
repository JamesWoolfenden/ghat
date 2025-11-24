package core

import (
	"os"
	"strings"
	"testing"

	"github.com/rs/zerolog/log"
)

func TestFlags_UpdateGitlab(t *testing.T) {
	type fields struct {
		File            string
		Directory       string
		GitHubToken     string
		Days            *uint
		DryRun          bool
		Entries         []string
		Update          bool
		ContinueOnError bool
	}

	var days uint = 1

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"Blank", fields{}, true},
		{"Empty",
			fields{"", "", gitHubToken, &days, false, nil, true, false}, true},
		{"Missing",
			fields{"", "testdata/gitlab/empty", gitHubToken, &days, false, nil, true, false}, true},
		{"Empty Project",
			fields{"", "testdata/gitlab/projectEmpty", gitHubToken, &days, true, nil, true, false}, true},
		{"Project",
			fields{"", "testdata/gitlab/simple", gitHubToken, &days, true, nil, true, false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			myFlags := &Flags{
				File:            tt.fields.File,
				Directory:       tt.fields.Directory,
				GitHubToken:     tt.fields.GitHubToken,
				Days:            tt.fields.Days,
				DryRun:          tt.fields.DryRun,
				Entries:         tt.fields.Entries,
				Update:          tt.fields.Update,
				ContinueOnError: tt.fields.ContinueOnError,
			}
			if err := myFlags.UpdateGitlab(); (err != nil) != tt.wantErr {
				t.Errorf("UpdateGitlab() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_gitlabProjectError_Error(t *testing.T) {
	type fields struct {
		directory string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"invoke", fields{"invoke"}, "gitlab project not found in directory: invoke"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &gitlabProjectError{
				directory: tt.fields.directory,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_gitlabProjectEmptyError_Error(t *testing.T) {
	type fields struct {
		file string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{"invoke", fields{"invoke"}, "gitlab project empty: invoke"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &gitlabProjectEmptyError{
				file: tt.fields.file,
			}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractImages(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
		wantErr bool
	}{
		{
			name: "simple image",
			content: `
stages:
  - test
test:
  image: golang:1.21
  script:
    - go test
`,
			want:    []string{"golang:1.21"},
			wantErr: false,
		},
		{
			name: "image with name",
			content: `
test:
  image:
    name: node:18-alpine
    entrypoint: [""]
  script:
    - npm test
`,
			want:    []string{"node:18-alpine"},
			wantErr: false,
		},
		{
			name: "multiple images",
			content: `
job1:
  image: golang:1.21
  script:
    - go build
job2:
  image: node:18
  script:
    - npm test
`,
			want:    []string{"golang:1.21", "node:18"},
			wantErr: false,
		},
		{
			name: "ignore variable images",
			content: `
test:
  image: $CI_REGISTRY_IMAGE
  script:
    - echo test
`,
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			content: "invalid: [yaml",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractImages(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(got) != len(tt.want) {
					t.Errorf("extractImages() got %d images, want %d. got=%v, want=%v", len(got), len(tt.want), got, tt.want)
					return
				}
				for i, img := range got {
					if img != tt.want[i] {
						t.Errorf("extractImages() got[%d] = %v, want %v", i, img, tt.want[i])
					}
				}
			}
		})
	}
}

func Test_parseImageReference(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  ImageReference
	}{
		{
			name:  "simple image",
			image: "golang:1.21",
			want: ImageReference{
				Registry:   "docker.io",
				Repository: "library/golang",
				Tag:        "1.21",
				Original:   "golang:1.21",
			},
		},
		{
			name:  "no tag",
			image: "nginx",
			want: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Original:   "nginx",
			},
		},
		{
			name:  "with registry",
			image: "gcr.io/project/image:v1.0",
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/image",
				Tag:        "v1.0",
				Original:   "gcr.io/project/image:v1.0",
			},
		},
		{
			name:  "docker hub user image",
			image: "jameswoolfenden/ghat:latest",
			want: ImageReference{
				Registry:   "docker.io",
				Repository: "jameswoolfenden/ghat",
				Tag:        "latest",
				Original:   "jameswoolfenden/ghat:latest",
			},
		},
		{
			name:  "with digest",
			image: "nginx@sha256:abcdef123456",
			want: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "latest",
				Digest:     "sha256:abcdef123456",
				Original:   "nginx@sha256:abcdef123456",
			},
		},
		{
			name:  "ghcr image",
			image: "ghcr.io/owner/repo:tag",
			want: ImageReference{
				Registry:   "ghcr.io",
				Repository: "owner/repo",
				Tag:        "tag",
				Original:   "ghcr.io/owner/repo:tag",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseImageReference(tt.image)
			if got.Registry != tt.want.Registry {
				t.Errorf("parseImageReference().Registry = %v, want %v", got.Registry, tt.want.Registry)
			}
			if got.Repository != tt.want.Repository {
				t.Errorf("parseImageReference().Repository = %v, want %v", got.Repository, tt.want.Repository)
			}
			if got.Tag != tt.want.Tag {
				t.Errorf("parseImageReference().Tag = %v, want %v", got.Tag, tt.want.Tag)
			}
			if got.Digest != tt.want.Digest {
				t.Errorf("parseImageReference().Digest = %v, want %v", got.Digest, tt.want.Digest)
			}
			if got.Original != tt.want.Original {
				t.Errorf("parseImageReference().Original = %v, want %v", got.Original, tt.want.Original)
			}
		})
	}
}

func Test_formatImageWithDigest(t *testing.T) {
	tests := []struct {
		name   string
		ref    ImageReference
		digest string
		want   string
	}{
		{
			name: "docker hub official image",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/golang",
				Tag:        "1.21",
			},
			digest: "sha256:abc123",
			want:   "golang@sha256:abc123 # 1.21",
		},
		{
			name: "docker hub user image",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "jameswoolfenden/ghat",
				Tag:        "latest",
			},
			digest: "sha256:def456",
			want:   "jameswoolfenden/ghat@sha256:def456 # latest",
		},
		{
			name: "custom registry",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/image",
				Tag:        "v1.0",
			},
			digest: "sha256:xyz789",
			want:   "gcr.io/project/image@sha256:xyz789 # v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatImageWithDigest(tt.ref, tt.digest)
			if got != tt.want {
				t.Errorf("formatImageWithDigest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetGitlabFiles(t *testing.T) {
	// Create a temporary test structure
	tmpDir, err := os.MkdirTemp("", "gitlab-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Warn().Err(err).Msg("failed to remove temporary directory")
		}
	}(tmpDir)

	// Create a .gitlab-ci.yml file
	gitlabFile := tmpDir + "/.gitlab-ci.yml"
	if err := os.WriteFile(gitlabFile, []byte("test: true"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		directory string
		entries   []string
		wantCount int
	}{
		{
			name:      "finds gitlab ci file",
			directory: tmpDir,
			entries:   []string{},
			wantCount: 1,
		},
		{
			name:      "finds in entries",
			directory: "/nonexistent",
			entries:   []string{"/path/to/.gitlab-ci.yml", "/path/to/.gitlab-ci.yaml"},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &Flags{
				Directory: tt.directory,
				Entries:   tt.entries,
			}
			got := flags.GetGitlabFiles()
			if len(got) != tt.wantCount {
				t.Errorf("GetGitlabFiles() returned %d files, want %d", len(got), tt.wantCount)
			}
		})
	}
}

// Test image parsing edge cases
func Test_parseImageReference_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		image string
	}{
		{"port in registry", "localhost:5000/myimage:tag"},
		{"multiple slashes", "registry.com/path/to/image:tag"},
		{"hyphenated names", "my-registry.io/my-org/my-image:my-tag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := parseImageReference(tt.image)
			// Just make sure it doesn't panic
			if ref.Original != tt.image {
				t.Errorf("Original image not preserved: got %v, want %v", ref.Original, tt.image)
			}
		})
	}
}

// Test that we skip variable references
func Test_findImages_SkipsVariables(t *testing.T) {
	content := `
test:
  image: $CI_REGISTRY_IMAGE
  script:
    - echo test
another:
  image: ${DOCKER_IMAGE}
  script:
    - echo test
valid:
  image: golang:1.21
  script:
    - go test
`
	images, err := extractImages(content)
	if err != nil {
		t.Fatal(err)
	}

	// Should only find the golang image, not the variables
	if len(images) != 1 {
		t.Errorf("Expected 1 image, got %d: %v", len(images), images)
	}

	if len(images) > 0 && !strings.Contains(images[0], "golang") {
		t.Errorf("Expected golang image, got %v", images[0])
	}
}
