package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zarathos364/opencode-memory/internal/storage"
)

func newService(t *testing.T) (*MemoryService, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), storage.OpenCodeDirName)

	opened, err := New(root)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := opened.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	})
	return opened, root
}

func toString(value string) *string { return &value }
func toBool(value bool) *bool       { return &value }
func toSlice(value []string) *[]string {
	return &value
}

func TestCreateThenSearchFindsTheMemory(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("ferret care", "how to look after a ferret", []string{"pets"}, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	results, err := svc.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(results))
	}
	if results[0].Memory.ID != created.ID {
		t.Errorf("result id = %q, want %q", results[0].Memory.ID, created.ID)
	}
	if results[0].Memory.Content != created.Content {
		t.Errorf("result content = %q, want %q", results[0].Memory.Content, created.Content)
	}
}

func TestCreateRefusesATitleAlreadyStored(t *testing.T) {
	svc, _ := newService(t)

	first, err := svc.Create("shell access", "the shell is allowed", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := svc.Create("Shell Access", "a second copy", nil, false); err == nil {
		t.Fatal("Create with a stored title returned nil error, want a refusal")
	} else if !strings.Contains(err.Error(), first.ID) {
		t.Errorf("the refusal does not name the stored memory: %v", err)
	}
}

func TestSearchPrefersTheCompleteMatch(t *testing.T) {
	svc, _ := newService(t)

	if _, err := svc.Create("shell access", "the shell is allowed on this machine", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Create("shell notes", "notes about the shell", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	complete, err := svc.Search("shell allowed", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(complete) != 1 {
		t.Fatalf("Search returned %d results, want the complete match only", len(complete))
	}
	if complete[0].Memory.Title != "shell access" {
		t.Errorf("result title = %q, want %q", complete[0].Memory.Title, "shell access")
	}
	if complete[0].Weak {
		t.Errorf("a result that matched every word is marked weak")
	}

	encoded, err := json.Marshal(NewMemorySummaryResults(complete))
	if err != nil {
		t.Fatalf("marshalling the summary failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"weak":false`) {
		t.Errorf("a complete match does not state weakness as false: %s", encoded)
	}

	withStopword, err := svc.Search("the shell allowed", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(withStopword) != 1 {
		t.Fatalf("Search with a common word returned %d results, want 1", len(withStopword))
	}
	if withStopword[0].Weak {
		t.Errorf("a common word turned a complete match into a weak one")
	}
}

func TestSearchMarksPartialMatchesAsWeak(t *testing.T) {
	svc, _ := newService(t)

	if _, err := svc.Create("shell notes", "notes about the shell", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	partial, err := svc.Search("shell command denied", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(partial) != 1 {
		t.Fatalf("Search returned %d results, want 1", len(partial))
	}
	if !partial[0].Weak {
		t.Errorf("a result that matched part of the query is not marked weak")
	}

	summaries := NewMemorySummaryResults(partial)
	if len(summaries) != 1 || !summaries[0].Weak {
		t.Errorf("the summary does not carry the weak mark")
	}
	encoded, err := json.Marshal(summaries)
	if err != nil {
		t.Fatalf("marshalling the summary failed: %v", err)
	}
	if !strings.Contains(string(encoded), `"weak":true`) {
		t.Errorf("the encoded summary does not carry the weak mark: %s", encoded)
	}
}

func TestSearchFallsBackWhenTheStrictHitIsGone(t *testing.T) {
	svc, root := newService(t)

	complete, err := svc.Create("shell access", "the shell is allowed", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Create("shell notes", "notes about the shell", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(storage.RawDir(root), complete.ID+".md")); err != nil {
		t.Fatalf("removing the file failed: %v", err)
	}

	results, err := svc.Search("shell allowed", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want the live partial match", len(results))
	}
	if results[0].Memory.Title != "shell notes" {
		t.Errorf("result title = %q, want %q", results[0].Memory.Title, "shell notes")
	}
	if !results[0].Weak {
		t.Errorf("the fallback result is not marked weak")
	}
}

func TestAHiddenTitleDoesNotAnswerTheUniquenessCheck(t *testing.T) {
	svc, _ := newService(t)

	if _, err := svc.Create("hidden fact", "secret body", nil, true); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if _, err := svc.Create("Hidden Fact", "public body", nil, false); err != nil {
		t.Errorf("Create with the title of a sensitive memory returned error: %v", err)
	}
}

func TestUpdateRefusesATitleAlreadyStored(t *testing.T) {
	svc, _ := newService(t)

	first, err := svc.Create("shell access", "the shell is allowed", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	second, err := svc.Create("shell notes", "notes about the shell", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	_, err = svc.Update(second.ID, Patch{Title: toString("  Shell Access  ")})
	if err == nil {
		t.Fatal("Update to a stored title returned nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), first.ID) {
		t.Errorf("the refusal does not name the other memory: %v", err)
	}

	if _, err := svc.Update(first.ID, Patch{Title: toString("Shell Access")}); err != nil {
		t.Errorf("Update to the memory's own title returned error: %v", err)
	}
}

func TestUpdateReplacesOnePassage(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "alpha beta gamma", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "beta", With: "BETA"}})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Content != "alpha BETA gamma" {
		t.Errorf("content = %q, want %q", updated.Content, "alpha BETA gamma")
	}

	results, err := svc.Search("BETA", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search returned %d results after the replacement, want 1", len(results))
	}
}

func TestUpdateRefusesAReplacementThatDoesNotFit(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "beta and beta again", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	for _, find := range []string{"absent", "beta", "  "} {
		if _, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: find, With: "x"}}); err == nil {
			t.Errorf("Update replacing %q returned nil error, want a refusal", find)
		}
	}
}

func TestUpdateMovesTheUpdateTime(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "alpha beta gamma", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !created.UpdatedAt.Equal(created.CreatedAt) {
		t.Errorf("a new memory has update time %v, want its creation time %v", created.UpdatedAt, created.CreatedAt)
	}

	updated, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "beta", With: "BETA"}})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Errorf("update time %v did not move past the creation time %v", updated.UpdatedAt, updated.CreatedAt)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("update moved the creation time to %v", updated.CreatedAt)
	}
}

func TestUpdateReplacesAPassageWithNothing(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "alpha beta gamma", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "beta ", With: ""}})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Content != "alpha gamma" {
		t.Errorf("content = %q, want %q", updated.Content, "alpha gamma")
	}
}

func TestUpdateReplacesAPassageAcrossLines(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "alpha\nbeta\ngamma", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "beta\ngamma", With: "delta"}})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.Content != "alpha\ndelta" {
		t.Errorf("content = %q, want %q", updated.Content, "alpha\ndelta")
	}
}

func TestUpdateRefusesToEmptyTheBody(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "alpha", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "alpha", With: ""}}); err == nil {
		t.Fatal("Update removing the whole body returned nil error, want a refusal")
	}
	stored, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.Content != "alpha" {
		t.Errorf("content = %q after a refused update, want it unchanged", stored.Content)
	}
}

func TestARefusedUpdateLeavesTheMemoryAlone(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("shell notes", "beta and beta again", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := svc.Update(created.ID, Patch{Replace: &Replacement{Find: "beta", With: "x"}}); err == nil {
		t.Fatal("Update with an ambiguous passage returned nil error, want a refusal")
	}

	stored, err := svc.Get(created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if stored.Content != created.Content || !stored.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("the refused update changed the memory: content %q, updated %v", stored.Content, stored.UpdatedAt)
	}
	results, err := svc.Search("beta", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search returned %d results after a refused update, want 1", len(results))
	}
	consistent, err := svc.Consistent()
	if err != nil {
		t.Fatalf("Consistent returned error: %v", err)
	}
	if !consistent {
		t.Error("the index drifted after a refused update")
	}
}

func TestTheSummaryCarriesTheUpdateTime(t *testing.T) {
	svc, _ := newService(t)

	if _, err := svc.Create("shell notes", "alpha beta gamma", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	memories, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if _, err := svc.Update(memories[0].ID, Patch{Replace: &Replacement{Find: "beta", With: "BETA"}}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	memories, err = svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	summaries := NewMemorySummaryList(memories)
	if len(summaries) != 1 {
		t.Fatalf("List returned %d memories, want 1", len(summaries))
	}
	if summaries[0].UpdatedAt == summaries[0].CreatedAt {
		t.Errorf("the summary still carries the creation time as its update time")
	}
}

func TestDeleteRemovesTheMemoryFromSearch(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("ferret care", "how to look after a ferret", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := svc.Delete(created.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	if _, err := svc.Get(created.ID); err == nil {
		t.Error("Get after delete returned nil error, want not found")
	}
	results, err := svc.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search returned %d results after delete, want 0", len(results))
	}
}

func TestSensitiveMemoriesAreStoredButNotSearchable(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("a secret", "the passphrase is hunter2", nil, true)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := svc.Get(created.ID); err != nil {
		t.Errorf("Get for a sensitive memory returned error: %v", err)
	}
	list, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List returned %d memories, want 1", len(list))
	}

	results, err := svc.Search("passphrase", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search returned %d results for a sensitive memory, want 0", len(results))
	}

	consistent, err := svc.Consistent()
	if err != nil {
		t.Fatalf("Consistent returned error: %v", err)
	}
	if !consistent {
		t.Error("Consistent = false after storing a sensitive memory, want true")
	}
}

func TestUpdateKeepsIdentityAndRefreshesSearch(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("alpha", "gamma", []string{"first"}, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	updated, err := svc.Update(created.ID, Patch{
		Title: toString("beta"),
		Tags:  toSlice([]string{"second"}),
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.ID != created.ID {
		t.Errorf("ID changed from %q to %q", created.ID, updated.ID)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt changed from %v to %v", created.CreatedAt, updated.CreatedAt)
	}
	if updated.Content != created.Content {
		t.Errorf("Content = %q, want it left alone", updated.Content)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "second" {
		t.Errorf("Tags = %v, want [second]", updated.Tags)
	}

	if results, err := svc.Search("alpha", 10); err != nil || len(results) != 0 {
		t.Errorf("Search for the replaced title returned %d results (err = %v), want 0", len(results), err)
	}
	if results, err := svc.Search("beta", 10); err != nil || len(results) != 1 {
		t.Errorf("Search for the new title returned %d results (err = %v), want 1", len(results), err)
	}
}

func TestUpdateCanTakeAMemoryOutOfSearch(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("ferret care", "how to look after a ferret", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if results, err := svc.Search("ferret", 10); err != nil || len(results) != 1 {
		t.Fatalf("Search before the update returned %d results (err = %v), want 1", len(results), err)
	}

	if _, err := svc.Update(created.ID, Patch{Sensitive: toBool(true)}); err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if results, err := svc.Search("ferret", 10); err != nil || len(results) != 0 {
		t.Errorf("Search after the update returned %d results (err = %v), want 0", len(results), err)
	}
	consistent, err := svc.Consistent()
	if err != nil {
		t.Fatalf("Consistent returned error: %v", err)
	}
	if !consistent {
		t.Error("Consistent = false after taking a memory out of search, want true")
	}
}

func TestReindexRebuildsFromTheStoredFiles(t *testing.T) {
	root := filepath.Join(t.TempDir(), storage.OpenCodeDirName)

	first, err := New(root)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	created, err := first.Create("ferret care", "how to look after a ferret", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if err := os.RemoveAll(storage.IndexDir(root)); err != nil {
		t.Fatalf("removing the index directory failed: %v", err)
	}

	svc, err := New(root)
	if err != nil {
		t.Fatalf("reopening the store returned error: %v", err)
	}
	defer func() {
		if err := svc.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	}()

	consistent, err := svc.Consistent()
	if err != nil {
		t.Fatalf("Consistent returned error: %v", err)
	}
	if consistent {
		t.Fatal("Consistent = true after deleting the index, want false")
	}

	stored, err := svc.Reindex()
	if err != nil {
		t.Fatalf("Reindex returned error: %v", err)
	}
	if stored != 1 {
		t.Errorf("Reindex stored %d memories, want 1", stored)
	}

	consistent, err = svc.Consistent()
	if err != nil {
		t.Fatalf("Consistent returned error: %v", err)
	}
	if !consistent {
		t.Error("Consistent = false after reindex, want true")
	}

	results, err := svc.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Memory.ID != created.ID {
		t.Errorf("Search after reindex returned %d results, want the stored memory", len(results))
	}
}

func TestSearchSkipsAHitWhoseFileIsGone(t *testing.T) {
	svc, root := newService(t)

	created, err := svc.Create("ferret care", "how to look after a ferret", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := os.Remove(filepath.Join(storage.RawDir(root), created.ID+".md")); err != nil {
		t.Fatalf("removing the file failed: %v", err)
	}

	results, err := svc.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search returned %d results for a missing file, want 0", len(results))
	}
}

func TestDeleteRefusesAnIdentifierThatIsNotAnIdentifier(t *testing.T) {
	svc, _ := newService(t)

	if err := svc.Delete("../../etc/passwd"); err == nil {
		t.Error("Delete with a path returned nil error, want rejection")
	}
}

func TestCallsOnAClosedServiceAreRefused(t *testing.T) {
	svc, _ := newService(t)
	if err := svc.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	calls := map[string]func() error{
		"Create":     func() error { _, err := svc.Create("title", "body", nil, false); return err },
		"Get":        func() error { _, err := svc.Get("0192f0c1-0000-7000-8000-000000000000"); return err },
		"List":       func() error { _, err := svc.List(); return err },
		"Search":     func() error { _, err := svc.Search("ferret", 10); return err },
		"Delete":     func() error { return svc.Delete("0192f0c1-0000-7000-8000-000000000000") },
		"Update":     func() error { _, err := svc.Update("0192f0c1-0000-7000-8000-000000000000", Patch{}); return err },
		"Reindex":    func() error { _, err := svc.Reindex(); return err },
		"Consistent": func() error { _, err := svc.Consistent(); return err },
	}
	for name, call := range calls {
		if err := call(); err == nil || err.Error() != "service is closed" {
			t.Errorf("%s after Close returned %v, want the closed service reported", name, err)
		}
	}

	if err := svc.Close(); err != nil {
		t.Errorf("second Close returned error: %v, want closing twice to be safe", err)
	}
}

func TestSearchSkipsAMemoryThatBecameSensitive(t *testing.T) {
	svc, _ := newService(t)

	created, err := svc.Create("ferret care", "how to look after a ferret", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	// Mark the file sensitive without touching the index, which is what a hand
	// edit leaves behind.
	created.Sensitive = true
	if err := svc.writer.Replace(created); err != nil {
		t.Fatalf("Replace returned error: %v", err)
	}

	results, err := svc.Search("ferret", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("search returned %d results, want none for a sensitive memory", len(results))
	}
}

func TestCreateNamesTheIdWhenTheIndexFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), storage.OpenCodeDirName)
	svc, err := New(root)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	// Close the index behind the service, so the next index write fails while
	// the service still looks open. No cleanup follows: the index is closed.
	if err := svc.index.Close(); err != nil {
		t.Fatalf("closing the index returned error: %v", err)
	}

	created, err := svc.Create("ferret care", "how to look after a ferret", nil, false)
	if err == nil {
		t.Fatal("Create returned nil error, want the index failure")
	}
	if created == nil || created.ID == "" {
		t.Fatal("Create returned no memory with the assigned id")
	}
	if !strings.Contains(err.Error(), created.ID) {
		t.Errorf("error = %q, want it to name the id %q", err, created.ID)
	}
}

func TestFailedReindexLeavesTheServiceClosed(t *testing.T) {
	root := filepath.Join(t.TempDir(), storage.OpenCodeDirName)

	svc, err := New(root)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(filepath.Join(root, storage.MemoriesDirName), 0o755); err != nil {
			t.Errorf("restoring permissions failed: %v", err)
		}
		if err := svc.Close(); err != nil {
			t.Errorf("Close returned error: %v", err)
		}
	})

	if _, err := svc.Create("ferret care", "how to look after a ferret", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	// Without write permission on the parent, removing the index directory
	// fails, which strands the service exactly like a failed reopen would.
	if err := os.Chmod(filepath.Join(root, storage.MemoriesDirName), 0o555); err != nil {
		t.Fatalf("removing write permission failed: %v", err)
	}
	if _, err := svc.Reindex(); err == nil {
		t.Fatal("Reindex without write permission returned nil error, want failure")
	}

	if _, err := svc.List(); err == nil || err.Error() != "service is closed" {
		t.Errorf("List after a failed Reindex returned %v, want the closed service reported", err)
	}
	if err := svc.Close(); err != nil {
		t.Errorf("Close after a failed Reindex returned error: %v, want no second close of the handle", err)
	}
}
