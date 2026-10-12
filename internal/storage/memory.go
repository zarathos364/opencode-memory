// Package storage persists memories as markdown files with a YAML header.
package storage

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"gopkg.in/yaml.v3"
)

const SchemaVersion = 1

// Memory is a stored memory: a versioned YAML header plus a markdown body.
type Memory struct {
	SchemaVersion int
	ID            string
	CreatedAt     time.Time
	Tags          []string
	Sensitive     bool
	Title         string
	Content       string
}

type header struct {
	SchemaVersion int      `yaml:"schema_version"`
	ID            string   `yaml:"id"`
	CreatedAt     string   `yaml:"created_at"`
	Tags          []string `yaml:"tags"`
	Sensitive     bool     `yaml:"sensitive"`
	Title         string   `yaml:"title"`
}

// Validate reports why a memory cannot be stored or was rejected on read.
func (m *Memory) Validate() error {
	if m == nil {
		return errors.New("memory is nil")
	}
	if m.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schema version %d", m.SchemaVersion)
	}
	if _, err := uuid.Parse(m.ID); err != nil {
		return fmt.Errorf("invalid id %q: %w", m.ID, err)
	}
	if m.CreatedAt.IsZero() {
		return errors.New("creation time is missing")
	}
	if strings.TrimSpace(m.Title) == "" {
		return errors.New("title is empty")
	}
	if strings.ContainsAny(m.Title, "\r\n") {
		return errors.New("title must stay on one line")
	}
	if strings.TrimSpace(m.Content) == "" {
		return errors.New("content is empty")
	}
	for _, tag := range m.Tags {
		if strings.TrimSpace(tag) == "" {
			return errors.New("a tag is empty")
		}
	}
	return nil
}

// NormalizeTags trims, lowercases and de-duplicates tags so that the stored value
// and the indexed value cannot drift apart.
func NormalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func (m *Memory) marshalFile() (string, error) {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	body, err := yaml.Marshal(header{
		SchemaVersion: m.SchemaVersion,
		ID:            m.ID,
		CreatedAt:     m.CreatedAt.UTC().Format(time.RFC3339Nano),
		Tags:          tags,
		Sensitive:     m.Sensitive,
		Title:         m.Title,
	})
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	return "---\n" + string(body) + "---\n\n" + m.Content + "\n", nil
}

func parseFile(file string) (*Memory, error) {
	const open = "---\n"
	if !strings.HasPrefix(file, open) {
		return nil, errors.New("header start marker is missing")
	}
	rest := file[len(open):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return nil, errors.New("header end marker is missing")
	}

	var h header
	if err := yaml.Unmarshal([]byte(rest[:end]), &h); err != nil {
		return nil, fmt.Errorf("parse header: %w", err)
	}
	created, err := time.Parse(time.RFC3339, h.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse creation time: %w", err)
	}

	// The writer stores a blank line after the header and one newline after the
	// body, so exactly one of each is removed to keep the body unchanged.
	body := strings.TrimPrefix(rest[end+len("\n---\n"):], "\n")
	body = strings.TrimSuffix(body, "\n")

	tags := h.Tags
	if tags == nil {
		tags = []string{}
	}
	m := &Memory{
		SchemaVersion: h.SchemaVersion,
		ID:            h.ID,
		CreatedAt:     created,
		Tags:          tags,
		Sensitive:     h.Sensitive,
		Title:         h.Title,
		Content:       body,
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}
