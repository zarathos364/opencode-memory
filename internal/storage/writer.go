package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"
)

type Writer struct {
	rawDir string
}

func NewWriter(rawDir string) *Writer {
	return &Writer{rawDir: rawDir}
}

// Create stores a new memory under a fresh identifier. The file is written to a
// temporary name and renamed only once its content is complete, so a reader never
// observes a half written memory and an interrupted run leaves no broken file.
func (w *Writer) Create(title, content string, tags []string, sensitive bool) (*Memory, error) {
	m := &Memory{
		SchemaVersion: SchemaVersion,
		ID:            uuid.NewV7().String(),
		CreatedAt:     time.Now().UTC(),
		Tags:          NormalizeTags(tags),
		Sensitive:     sensitive,
		Title:         strings.TrimSpace(title),
		Content:       content,
	}
	if err := w.store(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (w *Writer) Replace(m *Memory) error {
	return w.store(m)
}

func (w *Writer) Remove(id string) error {
	clean, err := CanonicalID(id)
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(w.rawDir, clean+".md")); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("memory %q not found", clean)
		}
		return fmt.Errorf("remove memory %q: %w", clean, err)
	}
	return nil
}

func (w *Writer) store(m *Memory) error {
	if err := m.Validate(); err != nil {
		return err
	}
	body, err := m.marshalFile()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.rawDir, 0o755); err != nil {
		return fmt.Errorf("create raw directory: %w", err)
	}
	// A sensitive memory stays out of the index but on the disk, so its file
	// must not stay readable by other users.
	mode := os.FileMode(0o644)
	if m.Sensitive {
		mode = 0o600
	}
	return writeAtomic(w.rawDir, m.ID+".md", body, mode)
}

func writeAtomic(dir, name, body string, mode os.FileMode) error {
	tmp, err := os.CreateTemp(dir, "mem-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("set file mode: %w", err)
	}
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("store file: %w", err)
	}
	return nil
}
