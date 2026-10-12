package service

import (
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
