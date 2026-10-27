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
	now := time.Now().UTC()
	m := &Memory{
		SchemaVersion: SchemaVersion,
		ID:            uuid.NewV7().String(),
		CreatedAt:     now,
		UpdatedAt:     now,
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

// Replace rewrites a stored memory and moves its update time forward, so a
// reader can tell a refreshed memory from one that predates a decision. The
// time never moves backwards: a clock that steps back keeps the previous value.
func (w *Writer) Replace(m *Memory) error {
	now := time.Now().UTC()
	floor := m.CreatedAt
	if m.UpdatedAt.After(floor) {
		floor = m.UpdatedAt
	}
	if now.Before(floor) {
		now = floor
	}
	m.UpdatedAt = now
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
	// The mode is requested at creation and the kernel applies the process
	// umask, so the operator's choice decides the final bits. 0666 is the
	// conventional base for a data file, the same base touch requests; a
	// sensitive memory asks for 0600 so it can never gain group or other
	// access.
	mode := os.FileMode(0o666)
	if m.Sensitive {
		mode = 0o600
	}
	return writeAtomic(w.rawDir, m.ID+".md", body, mode)
}

func writeAtomic(dir, name, body string, mode os.FileMode) error {
	tmp, err := createTemp(dir, name, mode)
	if err != nil {
		return err
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
	if err := os.Rename(tmpName, filepath.Join(dir, name)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("store file: %w", err)
	}
	return nil
}

// tempSuffix names the temporary file. It is a variable so a test can force a
// name collision.
var tempSuffix = func() string { return uuid.New().String() }

// createTemp opens a fresh temporary file beside the target, requesting the
// final mode so the kernel applies the umask at creation. os.CreateTemp cannot
// be used: it always creates with 0600 and would ignore the request.
func createTemp(dir, name string, mode os.FileMode) (*os.File, error) {
	for attempt := 0; attempt < 100; attempt++ {
		path := filepath.Join(dir, name+"."+tempSuffix()+".tmp")
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create temporary file: %w", err)
		}
	}
	return nil, errors.New("create temporary file: no free name")
}
