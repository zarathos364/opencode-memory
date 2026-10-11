package indexer

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// knownField marks input that already selects fields, so it is handed to Bleve
// unchanged instead of being rewritten as a plain term search.
var knownField = regexp.MustCompile(`(?i)\b(?:title|content|tags|sensitive|created_at|id):`)

// DefaultLimit bounds a search when the caller does not pick a limit.
const DefaultLimit = 10

// Search returns the best matches for raw, highest score first. Terms are matched
// against the title with more weight than against the body, while input that
// already names a field, such as a tag, keeps its own meaning.
func (i *Index) Search(raw string, limit int) ([]Hit, error) {
	terms := strings.TrimSpace(raw)
	if terms == "" {
		return nil, errors.New("search query is empty")
	}
	if limit <= 0 {
		limit = DefaultLimit
	}

	request := bleve.NewSearchRequest(buildQuery(terms))
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

func buildQuery(terms string) query.Query {
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
