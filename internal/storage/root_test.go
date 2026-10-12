package storage

import (
	"os"
	"path/filepath"
	"testing"
)

// storeAt creates a store that a walk up can recognise. The open code folder
// only counts as a store once it holds memories.
func storeAt(t *testing.T, base string) string {
	t.Helper()
	store := filepath.Join(base, OpenCodeDirName)
	if err := os.MkdirAll(filepath.Join(store, MemoriesDirName), 0o755); err != nil {
		t.Fatalf("creating the store failed: %v", err)
	}
	return store
}

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
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	base := t.TempDir()
	store := storeAt(t, base)
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

func TestResolveRootSkipsAConfigurationFolderWithoutMemories(t *testing.T) {
	t.Setenv(RootEnvVar, "")
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	base := t.TempDir()
	// A project carrying the open code folder but no memories is a setting, not
	// a store, so the shared store has to win over it.
	if err := os.Mkdir(filepath.Join(base, OpenCodeDirName), 0o755); err != nil {
		t.Fatalf("creating the configuration folder failed: %v", err)
	}

	got, err := ResolveRoot("", base)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	want := filepath.Join(configHome, OpenCodeDirName)
	if got != want {
		t.Errorf("ResolveRoot = %q, want the configuration directory %q", got, want)
	}
}

func TestResolveRootPrefersTheExplicitValue(t *testing.T) {
	t.Setenv(RootEnvVar, "")

	explicit := t.TempDir()
	storeAt(t, explicit)
	start := t.TempDir()
	storeAt(t, start)

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

func TestResolveRootFallsBackToTheConfigurationDirectory(t *testing.T) {
	t.Setenv(RootEnvVar, "")
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)

	got, err := ResolveRoot("", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	want := filepath.Join(configHome, OpenCodeDirName)
	if got != want {
		t.Errorf("ResolveRoot = %q, want the configuration directory %q", got, want)
	}
}

func TestResolveRootUsesTheHomeConfigurationDirectory(t *testing.T) {
	t.Setenv(RootEnvVar, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := ResolveRoot("", t.TempDir())
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	want := filepath.Join(home, ".config", OpenCodeDirName)
	if got != want {
		t.Errorf("ResolveRoot = %q, want the home configuration directory %q", got, want)
	}
}

func TestResolveRootFallsBackToTheWorkingDirectoryWithoutAHome(t *testing.T) {
	t.Setenv(RootEnvVar, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	start := t.TempDir()
	got, err := ResolveRoot("", start)
	if err != nil {
		t.Fatalf("ResolveRoot returned error: %v", err)
	}
	want := filepath.Join(start, OpenCodeDirName)
	if got != want {
		t.Errorf("ResolveRoot = %q, want the last fallback %q", got, want)
	}
}
