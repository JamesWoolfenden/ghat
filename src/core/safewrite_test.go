package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeWriteFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	t.Run("creates a new file", func(t *testing.T) {
		t.Parallel()

		p := filepath.Join(dir, "new.yml")
		if err := safeWriteFile(p, []byte("a"), 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("overwrites a regular file", func(t *testing.T) {
		t.Parallel()

		p := filepath.Join(dir, "existing.yml")
		if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := safeWriteFile(p, []byte("new"), 0o644); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, _ := os.ReadFile(p)
		if string(got) != "new" {
			t.Errorf("got %q, want %q", got, "new")
		}
	})

	t.Run("refuses a non-regular file", func(t *testing.T) {
		t.Parallel()

		d := filepath.Join(dir, "adir")
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := safeWriteFile(d, []byte("x"), 0o644); err == nil {
			t.Fatal("expected an error writing to a directory")
		}
	})

	t.Run("refuses a symlink and leaves the target alone", func(t *testing.T) {
		t.Parallel()

		target := filepath.Join(dir, "outside.txt")
		if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}

		link := filepath.Join(dir, "link.yml")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("symlinks not available: %v", err)
		}

		if err := safeWriteFile(link, []byte("pwned"), 0o644); err == nil {
			t.Fatal("expected an error writing through a symlink")
		}

		got, _ := os.ReadFile(target)
		if string(got) != "secret" {
			t.Errorf("symlink target was modified: %q", got)
		}
	})
}
