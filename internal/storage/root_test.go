package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsAreBuiltFromTheRoot(t *testing.T) {
	root := filepath.Join("some", OpenCodeDirName)

	raw := RawDir(root)
	if raw != filepath.Join(root, MemoriesDirName, RawDirName) {
		t.Errorf("RawDir = %q, want it inside the memories folder", raw)
	}
	index := IndexDir(root)
	if index != filepath.Join(root, MemoriesDirName, IndexDirName) {
		t.Errorf("IndexDir = %q, want it inside the memories folder", index)
	}
}

func TestResolveRootWalksUpToTheClosestStore(t *testing.T) {
	t.Setenv(RootEnvVar, "")

	base := t.TempDir()
	store := filepath.Join(base, OpenCodeDirName)
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatalf("creating the store failed: %v", err)
	}
	deep := filepath.Join(base, "one", "two")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatalf("creating the working folder failed: %v", err)
	}

	got, err := ResolveRoot("", deep)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	if got != store {
		t.Errorf("ResolveRoot = %q, want %q", got, store)
	}
}

func TestResolveRootPrefersTheExplicitValue(t *testing.T) {
	t.Setenv(RootEnvVar, "")

	explicit := t.TempDir()
	if err := os.Mkdir(filepath.Join(explicit, OpenCodeDirName), 0o755); err != nil {
		t.Fatalf("creating the store failed: %v", err)
	}
	start := t.TempDir()
	if err := os.Mkdir(filepath.Join(start, OpenCodeDirName), 0o755); err != nil {
		t.Fatalf("creating the store failed: %v", err)
	}

	got, err := ResolveRoot(explicit, start)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	if got != explicit {
		t.Errorf("ResolveRoot = %q, want the explicit root %q", got, explicit)
	}
}

func TestResolveRootUsesTheEnvironment(t *testing.T) {
	fromEnvironment := t.TempDir()
	start := t.TempDir()
	t.Setenv(RootEnvVar, fromEnvironment)

	got, err := ResolveRoot("", start)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	if got != fromEnvironment {
		t.Errorf("ResolveRoot = %q, want the root from the environment %q", got, fromEnvironment)
	}
}

func TestResolveRootRefusesAPathThatDoesNotExist(t *testing.T) {
	t.Setenv(RootEnvVar, "")

	missing := filepath.Join(t.TempDir(), "gone")
	if _, err := ResolveRoot(missing, t.TempDir()); err == nil {
		t.Error("ResolveRoot for a missing path returned nil error, want refusal")
	}
}

func TestResolveRootFallsBackToTheWorkingDirectory(t *testing.T) {
	t.Setenv(RootEnvVar, "")

	start := t.TempDir()
	got, err := ResolveRoot("", start)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	want := filepath.Join(start, OpenCodeDirName)
	if got != want {
		t.Errorf("ResolveRoot = %q, want the fallback %q", got, want)
	}
}
