package indexer

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

// knownField marks input that already selects fields, so it is handed to Bleve
// unchanged instead of being rewritten as a plain term search.
var knownField = regexp.MustCompile(`(?i)\b(?:title|content|tags|sensitive|created_at|id):`)

const DefaultLimit = 10

// IsFieldQuery reports whether the input selects fields itself, such as a tag.
// Such input keeps query-string semantics, so the strict pass has no meaning for
// it and the caller skips that pass.
func IsFieldQuery(terms string) bool {
	return knownField.MatchString(terms)
}

// Search returns the best matches for raw, highest score first. One matched
// term is enough, so the caller decides whether the rank means anything.
func (i *Index) Search(raw string, limit int) ([]Hit, error) {
	return i.search(buildLooseQuery, raw, limit)
}

// SearchAll returns only the memories that match every meaningful term in raw.
// It answers the question a ranked list cannot: does anything match all of what
// I typed? Input that names a field returns no hits; it keeps its own meaning.
func (i *Index) SearchAll(raw string, limit int) ([]Hit, error) {
	if err := i.checkQuery(raw); err != nil {
		return nil, err
	}
	if IsFieldQuery(raw) {
		return nil, nil
	}
	return i.search(i.buildStrictQuery, raw, limit)
}

func (i *Index) checkQuery(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("search query is empty")
	}
	return nil
}

func (i *Index) search(build func(string) query.Query, raw string, limit int) ([]Hit, error) {
	terms := strings.TrimSpace(raw)
	if terms == "" {
		return nil, errors.New("search query is empty")
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	request := bleve.NewSearchRequest(build(terms))
	request.Size = limit

	result, err := i.index.Search(request)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	hits := make([]Hit, 0, len(result.Hits))
	for _, match := range result.Hits {
		hits = append(hits, Hit{ID: match.ID, Score: match.Score})
	}
	return hits, nil
}

func buildLooseQuery(terms string) query.Query {
	if knownField.MatchString(terms) {
		return bleve.NewQueryStringQuery(terms)
	}

	title := bleve.NewMatchQuery(terms)
	title.SetField("title")
	title.SetBoost(2)

	body := bleve.NewMatchQuery(terms)
	body.SetField("content")

	return bleve.NewDisjunctionQuery(title, body)
}

// buildStrictQuery requires every term the index would store, each one in the
// title or in the body. A word the analyzer drops, such as a common article,
// yields no term and cannot be required.
func (i *Index) buildStrictQuery(terms string) query.Query {
	words := i.queryTerms(terms)
	if len(words) == 0 {
		return bleve.NewMatchNoneQuery()
	}

	clauses := make([]query.Query, 0, len(words))
	for _, word := range words {
		title := query.NewTermQuery(word)
		title.SetField("title")
		title.SetBoost(2)

		body := query.NewTermQuery(word)
		body.SetField("content")

		clauses = append(clauses, bleve.NewDisjunctionQuery(title, body))
	}
	return bleve.NewConjunctionQuery(clauses...)
}

// queryTerms splits raw the way the index does, so the strict pass requires
// exactly the terms a reader would call meaningful, not the raw whitespace.
func (i *Index) queryTerms(raw string) []string {
	implementation, ok := i.mapping.(*mapping.IndexMappingImpl)
	if !ok {
		return strings.Fields(raw)
	}
	stream, err := implementation.AnalyzeText(implementation.FieldAnalyzer("title"), []byte(raw))
	if err != nil {
		return strings.Fields(raw)
	}

	seen := make(map[string]struct{}, len(stream))
	terms := make([]string, 0, len(stream))
	for _, token := range stream {
		term := string(token.Term)
		if _, duplicate := seen[term]; duplicate {
			continue
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	return terms
}
