package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"uuid"
)

// Reader loads memories from one directory.
type Reader struct {
	rawDir string
}

// NewReader returns a reader rooted at rawDir, for example the raw folder of the
// memory store.
func NewReader(rawDir string) *Reader {
	return &Reader{rawDir: rawDir}
}

// Get loads one memory. Only an identifier understood by this package is
// accepted, so a caller can never reach a file outside the store.
func (r *Reader) Get(id string) (*Memory, error) {
	clean, err := CanonicalID(id)
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(filepath.Join(r.rawDir, clean+".md"))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("memory %q not found", clean)
		}
		return nil, fmt.Errorf("read memory %q: %w", clean, err)
	}
	return parseFile(string(body))
}

// List loads every stored memory ordered by creation. Identifiers are issued in
// creation order, so sorting them yields the chronological order.
func (r *Reader) List() ([]*Memory, error) {
	entries, err := os.ReadDir(r.rawDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read raw directory: %w", err)
	}

	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".md"))
	}
	slices.Sort(ids)

	out := make([]*Memory, 0, len(ids))
	for _, id := range ids {
		m, err := r.Get(id)
		if err != nil {
			// A file that is not a memory, or that no longer parses, must not
			// block the listing: that would also block the command which
			// rebuilds the index from these same files. The file stays on disk,
			// and an explicit get on it still reports the failure.
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// CanonicalID reduces an accepted identifier to the exact form used for file
// names. Anything the package cannot recognise is refused before it reaches the
// filesystem.
func CanonicalID(id string) (string, error) {
	id = strings.TrimSuffix(strings.TrimSpace(id), ".md")
	parsed, err := uuid.Parse(id)
	if err != nil {
		return "", fmt.Errorf("invalid memory id %q", id)
	}
	return parsed.String(), nil
}
