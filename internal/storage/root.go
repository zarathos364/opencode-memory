package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout of the memory store inside the open code directory.
const (
	OpenCodeDirName = ".opencode"
	MemoriesDirName = "memories"
	RawDirName      = "raw"
	IndexDirName    = "index"

	// RootEnvVar overrides the store location when no root is given.
	RootEnvVar = "OPENCODE_MEMORY_ROOT"
)

// RawDir is where the markdown files live under a root.
func RawDir(root string) string {
	return filepath.Join(root, MemoriesDirName, RawDirName)
}

// IndexDir is where the search index lives under a root.
func IndexDir(root string) string {
	return filepath.Join(root, MemoriesDirName, IndexDirName)
}

// ResolveRoot picks the store location. An explicit value wins, then the
// environment, then the closest ancestor that already holds a store, and finally
// the starting directory. A root that was named but is missing is refused, so a
// stale setting can never start a second store by accident.
func ResolveRoot(explicit, startDir string) (string, error) {
	if explicit != "" {
		return existingDir(explicit, "--root")
	}
	if fromEnvironment := os.Getenv(RootEnvVar); fromEnvironment != "" {
		return existingDir(fromEnvironment, RootEnvVar)
	}

	for dir := startDir; ; {
		candidate := filepath.Join(dir, OpenCodeDirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return absolute(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return absolute(filepath.Join(startDir, OpenCodeDirName))
}

func existingDir(path, source string) (string, error) {
	resolved, err := absolute(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("root from %s is not usable: %w", source, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("root from %s is not a directory: %s", source, resolved)
	}
	return resolved, nil
}

func absolute(path string) (string, error) {
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	return resolved, nil
}
