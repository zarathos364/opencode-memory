// Package service keeps the stored files and the search index in step.
package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/zarathos364/opencode-memory/internal/indexer"
	"github.com/zarathos364/opencode-memory/internal/storage"
)

type MemoryService struct {
	writer   *storage.Writer
	reader   *storage.Reader
	index    *indexer.Index
	indexDir string
	// closed tells whether the index is still held. A failed Reindex sets it,
	// so a later call reports a closed service instead of using a stale handle.
	closed bool
}

type Result struct {
	Memory *storage.Memory
	Score  float64
}

// MemoryJSON is one memory as data, for CLI output and tool results alike.
// Sharing it keeps both shapes from drifting apart.
type MemoryJSON struct {
	ID        string   `json:"id"`
	CreatedAt string   `json:"created_at"`
	Title     string   `json:"title"`
	Tags      []string `json:"tags"`
	Sensitive bool     `json:"sensitive"`
	Content   string   `json:"content"`
	Score     float64  `json:"score,omitempty"`
}

// NewMemoryJSON renders a memory with its creation time in UTC and an empty
// tag list instead of null, so every consumer prints the same bytes.
func NewMemoryJSON(m *storage.Memory, score float64) MemoryJSON {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	return MemoryJSON{
		ID:        m.ID,
		CreatedAt: m.CreatedAt.UTC().Format(time.RFC3339Nano),
		Title:     m.Title,
		Tags:      tags,
		Sensitive: m.Sensitive,
		Content:   m.Content,
		Score:     score,
	}
}

// NewMemoryJSONList renders memories in order, without scores.
func NewMemoryJSONList(memories []*storage.Memory) []MemoryJSON {
	out := make([]MemoryJSON, 0, len(memories))
	for _, m := range memories {
		out = append(out, NewMemoryJSON(m, 0))
	}
	return out
}

// NewMemoryJSONResults renders search results with their scores.
func NewMemoryJSONResults(results []Result) []MemoryJSON {
	out := make([]MemoryJSON, 0, len(results))
	for _, result := range results {
		out = append(out, NewMemoryJSON(result.Memory, result.Score))
	}
	return out
}

// VisibleMemories drops sensitive memories unless they were asked for.
func VisibleMemories(memories []*storage.Memory, showSensitive bool) []*storage.Memory {
	out := make([]*storage.Memory, 0, len(memories))
	for _, m := range memories {
		if m.Sensitive && !showSensitive {
			continue
		}
		out = append(out, m)
	}
	return out
}

// Patch lists the fields an update replaces. A field left nil keeps its value.
type Patch struct {
	Title     *string
	Content   *string
	Tags      *[]string
	Sensitive *bool
}

// AppendBody joins an addition to a stored body, separated by exactly one
// newline. Commands and tools share it so the separator cannot drift apart.
func AppendBody(current, addition string) string {
	if !strings.HasSuffix(current, "\n") {
		current += "\n"
	}
	return current + addition
}

// New opens the store under root and keeps the index locked until Close is called.
func New(root string) (*MemoryService, error) {
	rawDir := storage.RawDir(root)
	indexDir := storage.IndexDir(root)

	index, err := indexer.Open(indexDir)
	if err != nil {
		return nil, err
	}
	return &MemoryService{
		writer:   storage.NewWriter(rawDir),
		reader:   storage.NewReader(rawDir),
		index:    index,
		indexDir: indexDir,
	}, nil
}

// Close releases the index lock. Closing twice, or after a failed Reindex, is
// safe: only the first call touches the handle.
func (s *MemoryService) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return s.index.Close()
}

func (s *MemoryService) checkOpen() error {
	if s.closed {
		return errors.New("service is closed")
	}
	return nil
}

// Create stores a new memory. The file is written first, so an index failure
// leaves a memory that still exists and can be made searchable again later.
func (s *MemoryService) Create(title, content string, tags []string, sensitive bool) (*storage.Memory, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	m, err := s.writer.Create(title, content, tags, sensitive)
	if err != nil {
		return nil, err
	}
	if err := s.index.Index(toDocument(m)); err != nil {
		return m, fmt.Errorf("memory stored but not searchable, run reindex: %w", err)
	}
	return m, nil
}

func (s *MemoryService) Get(id string) (*storage.Memory, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	return s.reader.Get(id)
}

func (s *MemoryService) List() ([]*storage.Memory, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	return s.reader.List()
}

// Search returns the best matches, strongest first. A hit whose file is gone is
// skipped, because a stale index entry must not fail the whole search.
func (s *MemoryService) Search(query string, limit int) ([]Result, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	hits, err := s.index.Search(query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(hits))
	for _, hit := range hits {
		m, err := s.reader.Get(hit.ID)
		if err != nil {
			continue
		}
		results = append(results, Result{Memory: m, Score: hit.Score})
	}
	return results, nil
}

// Delete removes a memory. The file goes first, so an index failure leaves a
// stale entry instead of a memory that cannot be recovered.
func (s *MemoryService) Delete(id string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	canonical, err := storage.CanonicalID(id)
	if err != nil {
		return err
	}
	if err := s.writer.Remove(canonical); err != nil {
		return err
	}
	return s.index.Delete(canonical)
}

// Update rewrites a memory while keeping its identifier and its creation time.
func (s *MemoryService) Update(id string, patch Patch) (*storage.Memory, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	m, err := s.reader.Get(id)
	if err != nil {
		return nil, err
	}

	if patch.Title != nil {
		m.Title = strings.TrimSpace(*patch.Title)
	}
	if patch.Content != nil {
		m.Content = *patch.Content
	}
	if patch.Tags != nil {
		m.Tags = storage.NormalizeTags(*patch.Tags)
	}
	if patch.Sensitive != nil {
		m.Sensitive = *patch.Sensitive
	}

	if err := s.writer.Replace(m); err != nil {
		return nil, err
	}
	if err := s.index.Index(toDocument(m)); err != nil {
		return m, fmt.Errorf("memory stored but not searchable, run reindex: %w", err)
	}
	return m, nil
}

// Reindex rebuilds the index from the stored files. It is the single way back
// from a lost, damaged or drifted index, because the files are the source of truth.
// A failure closing, removing or reopening the index leaves the service closed.
// A failure while re-adding leaves the fresh index partial but usable, and
// Consistent reports the drift.
func (s *MemoryService) Reindex() (int, error) {
	if err := s.checkOpen(); err != nil {
		return 0, err
	}
	if err := s.index.Close(); err != nil {
		s.closed = true
		return 0, err
	}
	if err := os.RemoveAll(s.indexDir); err != nil {
		s.closed = true
		return 0, fmt.Errorf("remove index: %w", err)
	}

	index, err := indexer.Open(s.indexDir)
	if err != nil {
		s.closed = true
		return 0, err
	}
	s.index = index

	memories, err := s.reader.List()
	if err != nil {
		return 0, err
	}
	stored := 0
	for _, m := range memories {
		if err := s.index.Index(toDocument(m)); err != nil {
			return stored, fmt.Errorf("index memory %q: %w", m.ID, err)
		}
		if !m.Sensitive {
			stored++
		}
	}
	return stored, nil
}

// Consistent reports whether the index describes exactly the searchable part of
// the store. Sensitive memories are stored but never indexed, so they are not
// counted on either side.
func (s *MemoryService) Consistent() (bool, error) {
	memories, err := s.reader.List()
	if err != nil {
		return false, err
	}
	searchable := 0
	for _, m := range memories {
		if !m.Sensitive {
			searchable++
		}
	}

	count, err := s.index.Count()
	if err != nil {
		return false, err
	}
	return int(count) == searchable, nil
}

func toDocument(m *storage.Memory) indexer.Document {
	return indexer.Document{
		ID:        m.ID,
		Title:     m.Title,
		Content:   m.Content,
		Tags:      m.Tags,
		CreatedAt: m.CreatedAt,
		Sensitive: m.Sensitive,
	}
}
