package indexer

import (
	"fmt"
)

// Index stores one document. A sensitive memory is never stored: it is removed
// instead, so the index can never expose what the search must not reveal.
// Removal hides from search, not from forensics: old bytes linger until merge.
func (i *Index) Index(doc Document) error {
	if doc.Sensitive {
		return i.Delete(doc.ID)
	}
	if err := i.index.Index(doc.ID, doc); err != nil {
		return fmt.Errorf("index document %q: %w", doc.ID, err)
	}
	return nil
}

// Delete removes one document. Removing a document that is not stored is not an
// error, because that is the state callers already want.
func (i *Index) Delete(id string) error {
	if err := i.index.Delete(id); err != nil {
		return fmt.Errorf("remove document %q: %w", id, err)
	}
	return nil
}
