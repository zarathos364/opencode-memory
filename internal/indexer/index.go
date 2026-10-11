// Package indexer maintains a Bleve full-text index over stored memories.
package indexer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
)

// Document is the indexed projection of a memory.
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	Sensitive bool      `json:"sensitive"`
}

// Hit is a single search result.
type Hit struct {
	ID    string
	Score float64
}

// Index is one open handle over an index directory.
type Index struct {
	index bleve.Index
}

// LockError reports that another process already holds the index directory.
type LockError struct {
	Path string
	Err  error
}

func (e *LockError) Error() string {
	return fmt.Sprintf("open index: %s is held by another process: %v", e.Path, e.Err)
}

func (e *LockError) Unwrap() error {
	return e.Err
}

// IsLockError reports whether an error came from another process holding the index.
func IsLockError(err error) bool {
	var lockErr *LockError
	return errors.As(err, &lockErr)
}

// Open opens the index under dir and creates it on first use. Bleve keeps an
// exclusive lock on the directory, so a second process opening it at the same
// time is reported as a lock rather than as a storage failure.
func Open(dir string) (*Index, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create index directory: %w", err)
	}
	path := filepath.Join(dir, "index.bleve")

	var (
		index bleve.Index
		err   error
	)
	if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
		index, err = bleve.New(path, newIndexMapping())
	} else {
		index, err = bleve.Open(path)
	}
	if err != nil {
		if isLocked(err) {
			return nil, &LockError{Path: path, Err: err}
		}
		return nil, fmt.Errorf("open index: %w", err)
	}
	return &Index{index: index}, nil
}

// Close releases the index lock.
func (i *Index) Close() error {
	if err := i.index.Close(); err != nil {
		return fmt.Errorf("close index: %w", err)
	}
	return nil
}

// Count reports how many documents the index currently holds.
func (i *Index) Count() (uint64, error) {
	count, err := i.index.DocCount()
	if err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	return count, nil
}

func isLocked(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"file already locked",
		"resource temporarily unavailable",
		"timeout",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func newIndexMapping() mapping.IndexMapping {
	title := bleve.NewTextFieldMapping()
	content := bleve.NewTextFieldMapping()
	tags := bleve.NewKeywordFieldMapping()
	createdAt := bleve.NewDateTimeFieldMapping()
	sensitive := bleve.NewBooleanFieldMapping()
	id := bleve.NewKeywordFieldMapping()

	document := bleve.NewDocumentMapping()
	document.AddFieldMappingsAt("title", title)
	document.AddFieldMappingsAt("content", content)
	document.AddFieldMappingsAt("tags", tags)
	document.AddFieldMappingsAt("created_at", createdAt)
	document.AddFieldMappingsAt("sensitive", sensitive)
	document.AddFieldMappingsAt("id", id)

	index := bleve.NewIndexMapping()
	index.DefaultMapping = document
	return index
}
