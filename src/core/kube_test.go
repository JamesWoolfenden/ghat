package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_extractKubeImages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
		wantErr bool
	}{
		{
			name: "deployment with containers and initContainers",
			content: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      initContainers:
        - name: init
          image: busybox:1.36
      containers:
        - name: app
          image: nginx:1.25
`,
			want: []string{"busybox:1.36", "nginx:1.25"},
		},
		{
			name: "multi-document YAML",
			content: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.25
---
apiVersion: batch/v1
kind: Job
spec:
  template:
    spec:
      containers:
        - name: job
          image: alpine:3.18
`,
			want: []string{"alpine:3.18", "nginx:1.25"},
		},
		{
			name: "skips variable references",
			content: `
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: app
          image: $(IMAGE_TAG)
        - name: real
          image: nginx:1.25
`,
			want: []string{"nginx:1.25"},
		},
		{
			name: "cronjob nested spec",
			content: `
apiVersion: batch/v1
kind: CronJob
spec:
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: worker
              image: alpine:3.18
`,
			want: []string{"alpine:3.18"},
		},
		{
			name:    "invalid yaml",
			content: "invalid: [yaml",
			wantErr: true,
		},
		{
			name:    "no images",
			content: "apiVersion: v1\nkind: ConfigMap\ndata:\n  key: value\n",
			want:    []string{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractKubeImages(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractKubeImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("extractKubeImages() got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractKubeImages()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func Test_hasKubeResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"deployment", "apiVersion: apps/v1\nkind: Deployment\n", true},
		{"statefulset", "apiVersion: apps/v1\nkind: StatefulSet\n", true},
		{"daemonset", "apiVersion: apps/v1\nkind: DaemonSet\n", true},
		{"job", "apiVersion: batch/v1\nkind: Job\n", true},
		{"cronjob", "apiVersion: batch/v1\nkind: CronJob\n", true},
		{"pod", "apiVersion: v1\nkind: Pod\n", true},
		{"configmap — not a workload", "apiVersion: v1\nkind: ConfigMap\n", false},
		{"gitlab ci — no apiVersion", "stages:\n  - test\n", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasKubeResource(tt.content); got != tt.want {
				t.Errorf("hasKubeResource() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isKubeManifest(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	writeFile := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	kubeFile := writeFile("deploy.yaml", "apiVersion: apps/v1\nkind: Deployment\n")
	notKube := writeFile("config.yaml", "key: value\n")
	notYAML := writeFile("script.sh", "#!/bin/bash\n")

	tests := []struct {
		name string
		file string
		want bool
	}{
		{"kube manifest", kubeFile, true},
		{"non-kube yaml", notKube, false},
		{"non-yaml file", notYAML, false},
		{"nonexistent", filepath.Join(dir, "missing.yaml"), false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isKubeManifest(tt.file); got != tt.want {
				t.Errorf("isKubeManifest(%q) = %v, want %v", tt.file, got, tt.want)
			}
		})
	}
}

func Test_isKubeVarRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"$(IMAGE_TAG)", true},
		{"$IMAGE", true},
		{"nginx:1.25", false},
		{"gcr.io/foo/bar:v1", false},
		{"", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			if got := isKubeVarRef(tt.input); got != tt.want {
				t.Errorf("isKubeVarRef(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestGetKubeFiles verifies that GetKubeFiles filters entries to kube manifests only.
func TestGetKubeFiles(t *testing.T) {
	dir := t.TempDir()

	kubeContent := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: nginx:1.25\n"
	kubeFile := filepath.Join(dir, "deploy.yaml")
	otherFile := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(kubeFile, []byte(kubeContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherFile, []byte("key: value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{Entries: []string{kubeFile, otherFile}}
	got := flags.GetKubeFiles()

	if len(got) != 1 || got[0] != kubeFile {
		t.Errorf("GetKubeFiles() = %v, want [%s]", got, kubeFile)
	}
}

// TestUpdateKube_DryRun verifies dry-run leaves the file unchanged and produces a diff.
func TestUpdateKube_DryRun(t *testing.T) {
	original := "apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: app\n          image: nginx:1.25\n"

	dir := t.TempDir()
	file := filepath.Join(dir, "deploy.yaml")
	if err := os.WriteFile(file, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{DryRun: true, ContinueOnError: true}
	// continue-on-error so registry failures don't abort (no network in unit tests)
	if err := flags.UpdateKube(file); err != nil {
		t.Errorf("UpdateKube() unexpected error: %v", err)
	}

	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Error("UpdateKube() in dry-run mode modified the file")
	}
}

// TestUpdateKube_NoImages verifies files without container images are handled cleanly.
func TestUpdateKube_NoImages(t *testing.T) {
	content := "apiVersion: v1\nkind: ConfigMap\ndata:\n  key: value\n"

	dir := t.TempDir()
	file := filepath.Join(dir, "cm.yaml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{DryRun: true}
	if err := flags.UpdateKube(file); err != nil {
		t.Errorf("UpdateKube() unexpected error: %v", err)
	}
}

// TestUpdateKube_MultiDoc verifies multi-document YAML is accepted without error.
func TestUpdateKube_MultiDoc(t *testing.T) {
	content := strings.Join([]string{
		"apiVersion: apps/v1\nkind: Deployment\nspec:\n  template:\n    spec:\n      containers:\n        - name: web\n          image: nginx:1.25",
		"apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      containers:\n        - name: job\n          image: alpine:3.18",
	}, "\n---\n")

	dir := t.TempDir()
	file := filepath.Join(dir, "multi.yaml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	flags := &Flags{DryRun: true, ContinueOnError: true}
	if err := flags.UpdateKube(file); err != nil {
		t.Errorf("UpdateKube() unexpected error: %v", err)
	}
}
