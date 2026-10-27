package indexer

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newTestIndex(t *testing.T) *Index {
	t.Helper()
	opened, err := Open(filepath.Join(t.TempDir(), "index"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	})
	return opened
}

func sampleDocument(id, title, content string, tags ...string) Document {
	return Document{
		ID:        id,
		Title:     title,
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now().UTC(),
	}
}

func TestSearchRanksATitleMatchAboveABodyMatch(t *testing.T) {
	index := newTestIndex(t)

	for _, doc := range []Document{
		sampleDocument("title", "ferret care", "nothing relevant here"),
		sampleDocument("body", "unrelated subject", "mentions ferret once"),
	} {
		if err := index.Index(doc); err != nil {
			t.Fatalf("Index returned error: %v", err)
		}
	}

	hits, err := index.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("Search returned %d hits, want 2", len(hits))
	}
	if hits[0].ID != "title" {
		t.Errorf("top hit = %q, want %q", hits[0].ID, "title")
	}
	if hits[0].Score <= hits[1].Score {
		t.Errorf("title score %f is not above body score %f", hits[0].Score, hits[1].Score)
	}
}

func TestSearchAllRequiresEveryTerm(t *testing.T) {
	index := newTestIndex(t)

	for _, doc := range []Document{
		sampleDocument("both", "shell command denied", "the command was denied by the shell"),
		sampleDocument("across", "shell access", "the command needs it"),
		sampleDocument("one", "shell access", "the shell is allowed"),
	} {
		if err := index.Index(doc); err != nil {
			t.Fatalf("Index returned error: %v", err)
		}
	}

	strict, err := index.SearchAll("shell command", 10)
	if err != nil {
		t.Fatalf("SearchAll returned error: %v", err)
	}
	if !holdsExactly(strict, "both", "across") {
		t.Fatalf("SearchAll returned %v, want the memories holding both terms, across fields too", strict)
	}

	// A common word carries no term, so it cannot make the strict pass empty.
	withStopword, err := index.SearchAll("shell the command", 10)
	if err != nil {
		t.Fatalf("SearchAll returned error: %v", err)
	}
	if !holdsExactly(withStopword, "both", "across") {
		t.Fatalf("SearchAll with a common word returned %v, want the same two memories", withStopword)
	}

	loose, err := index.Search("shell command", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(loose) != 3 {
		t.Fatalf("Search returned %d hits, want 3", len(loose))
	}
}

// holdsExactly reports whether the hits name exactly these identifiers.
func holdsExactly(hits []Hit, ids ...string) bool {
	if len(hits) != len(ids) {
		return false
	}
	seen := make(map[string]bool, len(hits))
	for _, hit := range hits {
		seen[hit.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			return false
		}
	}
	return true
}

func TestTagSearchIsExactAndDoesNotStem(t *testing.T) {
	index := newTestIndex(t)

	if err := index.Index(sampleDocument("a", "a note", "an unrelated body", "go")); err != nil {
		t.Fatalf("Index returned error: %v", err)
	}

	hits, err := index.Search("tags:go", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "a" {
		t.Fatalf("Search(tags:go) returned %v, want the tagged memory", hits)
	}

	for _, terms := range []string{"gopher", "tags:gopher"} {
		hits, err := index.Search(terms, 10)
		if err != nil {
			t.Fatalf("Search(%q) returned error: %v", terms, err)
		}
		if len(hits) != 0 {
			t.Errorf("Search(%q) returned %d hits, want 0", terms, len(hits))
		}
	}
}

func TestSensitiveMemoriesNeverReachTheIndex(t *testing.T) {
	index := newTestIndex(t)

	if err := index.Index(sampleDocument("a", "title", "content")); err != nil {
		t.Fatalf("Index returned error: %v", err)
	}
	stored, err := index.Count()
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if stored != 1 {
		t.Fatalf("Count = %d after storing, want 1", stored)
	}

	// Re-storing the same memory as sensitive must take it back out.
	sensitive := sampleDocument("a", "title", "content")
	sensitive.Sensitive = true
	if err := index.Index(sensitive); err != nil {
		t.Fatalf("Index returned error: %v", err)
	}

	stored, err = index.Count()
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if stored != 0 {
		t.Errorf("Count = %d for a sensitive memory, want 0", stored)
	}
	hits, err := index.Search("title", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Search returned %d hits for a sensitive memory, want 0", len(hits))
	}
}

func TestDeleteRemovesAMemoryFromTheResults(t *testing.T) {
	index := newTestIndex(t)

	if err := index.Index(sampleDocument("a", "ferret", "care guide")); err != nil {
		t.Fatalf("Index returned error: %v", err)
	}
	if err := index.Delete("a"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if err := index.Delete("missing"); err != nil {
		t.Errorf("Delete for a memory that is not stored returned error: %v", err)
	}

	hits, err := index.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Search returned %d hits after delete, want 0", len(hits))
	}
}

func TestSearchRejectsEmptyInput(t *testing.T) {
	index := newTestIndex(t)

	for _, terms := range []string{"", "   "} {
		if _, err := index.Search(terms, 10); err == nil {
			t.Errorf("Search(%q) returned nil error, want rejection", terms)
		}
	}
}

func TestIndexSurvivesReopening(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "index")

	first, err := Open(dir)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := first.Index(sampleDocument("a", "ferret", "care guide")); err != nil {
		t.Fatalf("Index returned error: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	second, err := Open(dir)
	if err != nil {
		t.Fatalf("reopening the index returned error: %v", err)
	}
	defer func() {
		if err := second.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}()

	hits, err := second.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("Search after reopening returned %d hits, want 1", len(hits))
	}
}

func TestLockErrorsAreRecognised(t *testing.T) {
	cases := []struct {
		message string
		locked  bool
	}{
		{message: "file already locked", locked: true},
		{message: "Resource temporarily unavailable", locked: true},
		{message: "acquire lock: timeout", locked: true},
		{message: "disk is full", locked: false},
	}

	for _, tc := range cases {
		if got := isLocked(errors.New(tc.message)); got != tc.locked {
			t.Errorf("isLocked(%q) = %t, want %t", tc.message, got, tc.locked)
		}
	}
	if isLocked(nil) {
		t.Error("isLocked(nil) = true, want false")
	}
	if !IsLockError(&LockError{Path: "index.bleve", Err: errors.New("file already locked")}) {
		t.Error("IsLockError for a lock error = false, want true")
	}
	if IsLockError(errors.New("disk is full")) {
		t.Error("IsLockError for an unrelated error = true, want false")
	}
}

func TestEmptyIndexStartsWithNoResults(t *testing.T) {
	index := newTestIndex(t)

	count, err := index.Count()
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("Count = %d, want 0", count)
	}

	hits, err := index.Search("anything", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("Search returned %d hits on an empty index, want 0", len(hits))
	}
}

func TestLimitBoundsTheNumberOfHits(t *testing.T) {
	index := newTestIndex(t)

	for n := 0; n < 15; n++ {
		if err := index.Index(sampleDocument(fmt.Sprintf("memory-%02d", n), "ferret care guide", "ferret notes")); err != nil {
			t.Fatalf("Index returned error: %v", err)
		}
	}

	limited, err := index.Search("ferret", 3)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(limited) != 3 {
		t.Errorf("Search with limit 3 returned %d hits, want 3", len(limited))
	}

	defaulted, err := index.Search("ferret", 0)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(defaulted) != DefaultLimit {
		t.Errorf("Search without a limit returned %d hits, want %d", len(defaulted), DefaultLimit)
	}
}
