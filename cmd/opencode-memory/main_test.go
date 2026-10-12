package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zarathos364/opencode-memory/internal/service"
)

// rooted puts the store flag right after the command so that a test about the
// order of the other flags only measures the flag it is about.
func rooted(root, command string, rest ...string) []string {
	return append([]string{command, "--root", root}, rest...)
}

type cliResult struct {
	out    string
	errOut string
	err    error
}

func call(args ...string) cliResult {
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	err := run(args, out, errOut)
	return cliResult{out: out.String(), errOut: errOut.String(), err: err}
}

func createMemory(t *testing.T, root, title, body string) string {
	t.Helper()
	created := call(rooted(root, "add", "--title", title, body)...)
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}
	return strings.TrimSpace(created.out)
}

// storedBody reads one memory back as data, which is also the only way to see a
// body exactly as it was stored.
func storedBody(t *testing.T, root, id string) string {
	t.Helper()
	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	var one service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &one); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	return one.Content
}

func TestUpdateAcceptsFlagsAfterTheIdentifier(t *testing.T) {
	root := t.TempDir()
	id := createMemory(t, root, "Original title", "original body")

	updated := call(rooted(root, "update", id, "--title", "Replaced title", "replaced body")...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	shown := call(rooted(root, "get", id)...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	if !strings.Contains(shown.out, "title: Replaced title") {
		t.Errorf("get did not show the new title:\n%s", shown.out)
	}
	if !strings.Contains(shown.out, "replaced body") {
		t.Errorf("get did not show the new body:\n%s", shown.out)
	}
	if strings.Contains(shown.out, "--title") {
		t.Errorf("the flag was written into the body:\n%s", shown.out)
	}
}

func TestSearchAcceptsJSONAfterTheQuery(t *testing.T) {
	root := t.TempDir()
	createMemory(t, root, "Zumbido memory", "the marker word zumbido lives here")

	result := call(rooted(root, "search", "zumbido", "--json")...)
	if result.err != nil {
		t.Fatalf("search returned error: %v (%s)", result.err, result.errOut)
	}

	var hits []service.MemorySummary
	if err := json.Unmarshal([]byte(result.out), &hits); err != nil {
		t.Fatalf("search did not print JSON: %v\n%s", err, result.out)
	}
	if len(hits) != 1 {
		t.Errorf("search returned %d hits, want 1", len(hits))
	}
	if strings.Contains(result.out, "the marker word zumbido lives here") {
		t.Errorf("a search hit carried the body:\n%s", result.out)
	}
}

func TestSearchReadsALimitWrittenAfterTheQuery(t *testing.T) {
	root := t.TempDir()
	createMemory(t, root, "First zumbido", "zumbido appears here")
	createMemory(t, root, "Second zumbido", "zumbido appears again")

	result := call(rooted(root, "search", "zumbido", "--limit", "1")...)
	if result.err != nil {
		t.Fatalf("search returned error: %v (%s)", result.err, result.errOut)
	}
	if hits := strings.Count(result.out, "id="); hits != 1 {
		t.Errorf("search returned %d hits, want 1:\n%s", hits, result.out)
	}
}

func TestGetAcceptsTheStoreFlagAfterTheIdentifier(t *testing.T) {
	root := t.TempDir()
	id := createMemory(t, root, "Stored memory", "marker word trinado")

	shown := call("get", id, "--root", root)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	if !strings.Contains(shown.out, "title: Stored memory") {
		t.Errorf("get did not print the memory:\n%s", shown.out)
	}
}

func TestAddAcceptsSensitiveAfterTheText(t *testing.T) {
	root := t.TempDir()

	created := call("add", "--root", root, "--title", "Hidden memory", "secret marker word sigilo", "--sensitive")
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}

	visible := call(rooted(root, "list")...)
	if strings.Contains(visible.out, "Hidden memory") {
		t.Errorf("list showed a sensitive memory:\n%s", visible.out)
	}

	found := call(rooted(root, "search", "sigilo")...)
	if found.err != nil {
		t.Fatalf("search returned error: %v (%s)", found.err, found.errOut)
	}
	if strings.TrimSpace(found.out) != "" {
		t.Errorf("a sensitive memory reached the index:\n%s", found.out)
	}
}

// TestARoundTripThroughTheCLI walks add, search, get and delete through the
// command layer with a real store.
func TestARoundTripThroughTheCLI(t *testing.T) {
	root := t.TempDir()

	created := call(rooted(root, "add", "--title", "Ferret care", "how to look after a ferret")...)
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}
	id := strings.TrimSpace(created.out)

	found := call(rooted(root, "search", "ferret", "--json")...)
	if found.err != nil {
		t.Fatalf("search returned error: %v (%s)", found.err, found.errOut)
	}
	var hits []service.MemoryJSON
	if err := json.Unmarshal([]byte(found.out), &hits); err != nil {
		t.Fatalf("search did not print JSON: %v\n%s", err, found.out)
	}
	if len(hits) != 1 || hits[0].ID != id {
		t.Fatalf("search returned %v, want the memory %s", hits, id)
	}

	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	var memory service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &memory); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	if memory.Content != "how to look after a ferret" {
		t.Errorf("content = %q, want the stored body", memory.Content)
	}

	deleted := call(rooted(root, "delete", id)...)
	if deleted.err != nil {
		t.Fatalf("delete returned error: %v (%s)", deleted.err, deleted.errOut)
	}
	if strings.TrimSpace(deleted.out) != "deleted "+id {
		t.Errorf("delete printed %q, want the confirmation", deleted.out)
	}
}

func TestUpdateClearsSensitiveWithAFalseFlag(t *testing.T) {
	root := t.TempDir()
	created := call(rooted(root, "add", "--title", "Hidden memory", "secret marker word sigilo", "--sensitive")...)
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}
	id := strings.TrimSpace(created.out)

	updated := call(rooted(root, "update", "--sensitive=false", id)...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	var memory service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &memory); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	if memory.Sensitive {
		t.Error("sensitive = true after --sensitive=false, want false")
	}
}

func TestUnknownFlagIsStillReported(t *testing.T) {
	root := t.TempDir()

	result := call(rooted(root, "list", "--nope")...)
	if result.err == nil {
		t.Fatal("list accepted an unknown flag")
	}
	if !strings.Contains(result.err.Error(), "not defined") {
		t.Errorf("error = %q, want the message from the flag package", result.err)
	}
}

func TestHelpFlagAfterAPositional(t *testing.T) {
	root := t.TempDir()

	result := call(rooted(root, "search", "zumbido", "--help")...)
	if result.err != nil {
		t.Fatalf("search --help returned error: %v (%s)", result.err, result.errOut)
	}
	if !strings.Contains(result.errOut, "Usage of search") {
		t.Errorf("the flag usage was not printed:\n%s", result.errOut)
	}
}

func TestVersionPrintsTheLinkedValue(t *testing.T) {
	result := call("--version")
	if result.err != nil {
		t.Fatalf("--version returned error: %v (%s)", result.err, result.errOut)
	}
	if !strings.Contains(result.out, version) {
		t.Errorf("--version printed %q, want %q", result.out, version)
	}
}

func TestHelpListsTheCommands(t *testing.T) {
	result := call("help")
	if result.err != nil {
		t.Fatalf("help returned error: %v (%s)", result.err, result.errOut)
	}
	for _, want := range []string{"Usage: opencode-memory COMMAND", "  search ", "  reindex "} {
		if !strings.Contains(result.out, want) {
			t.Errorf("usage is missing %q:\n%s", want, result.out)
		}
	}
}

func TestNoCommandGivenPrintsTheUsage(t *testing.T) {
	result := call()
	if result.err == nil {
		t.Fatal("run without a command returned nil error")
	}
	if !strings.Contains(result.errOut, "Usage: opencode-memory") {
		t.Errorf("the usage was not printed to stderr:\n%s", result.errOut)
	}
}

func TestUnknownCommandIsReported(t *testing.T) {
	result := call("bogus")
	if result.err == nil {
		t.Fatal("run accepted an unknown command")
	}
	if !strings.Contains(result.err.Error(), "unknown command") {
		t.Errorf("error = %q, want it to name the command", result.err)
	}
}

func TestListPrintsJSON(t *testing.T) {
	root := t.TempDir()
	createMemory(t, root, "Listed memory", "the marker word listado lives here")

	result := call(rooted(root, "list", "--json")...)
	if result.err != nil {
		t.Fatalf("list returned error: %v (%s)", result.err, result.errOut)
	}

	var listed []service.MemorySummary
	if err := json.Unmarshal([]byte(result.out), &listed); err != nil {
		t.Fatalf("list did not print JSON: %v\n%s", err, result.out)
	}
	if len(listed) != 1 {
		t.Errorf("list returned %d memories, want 1", len(listed))
	}
	if strings.Contains(result.out, "the marker word listado lives here") {
		t.Errorf("a list entry carried the body:\n%s", result.out)
	}
}

func TestSplitTags(t *testing.T) {
	if got := strings.Join(splitTags("alpha,beta"), "|"); got != "alpha|beta" {
		t.Errorf("splitTags returned %q, want the two tags", got)
	}
	if got := splitTags(""); got != nil {
		t.Errorf("splitTags returned %v for an empty string, want nil", got)
	}
	if got := splitTags("   "); got != nil {
		t.Errorf("splitTags returned %v for whitespace, want nil", got)
	}
}

func TestOpenServiceRejectsAMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "missing")

	result := call("list", "--root", root)
	if result.err == nil {
		t.Fatal("list accepted a root that does not exist")
	}
	if !strings.Contains(result.err.Error(), "--root") {
		t.Errorf("error = %q, want it to name the source of the root", result.err)
	}
}

func TestOpenServiceNamesTheRealCause(t *testing.T) {
	root := t.TempDir()
	// A file where the index directory must go makes the creation step fail.
	if err := os.WriteFile(filepath.Join(root, "memories"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write the blocking file: %v", err)
	}

	result := call("list", "--root", root)
	if result.err == nil {
		t.Fatal("list accepted a root whose index cannot be created")
	}
	if !strings.Contains(result.err.Error(), "create the search index") {
		t.Errorf("error = %q, want it to name the failing step", result.err)
	}
	if strings.Contains(result.err.Error(), "remove") {
		t.Errorf("error = %q, want no advice about removing a directory that does not exist", result.err)
	}
}

func bodyFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "body.md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write the body file: %v", err)
	}
	return path
}

func TestGetPrintsJSONWithTheWholeBody(t *testing.T) {
	root := t.TempDir()
	body := "first line\nsecond line\n\n"
	id := createMemory(t, root, "Json memory", body)

	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}

	var one service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &one); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	if one.Content != body {
		t.Errorf("content = %q, want the body byte for byte, %q", one.Content, body)
	}
	if strings.Contains(shown.out, "score") {
		t.Errorf("get printed a score, which only a search has:\n%s", shown.out)
	}
}

func TestUpdateReadsTheBodyFromAFile(t *testing.T) {
	root := t.TempDir()
	created := call(rooted(root, "add", "--title", "Kept memory", "--tags", "go,arch",
		"the old marker zumbido lives here")...)
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}
	id := strings.TrimSpace(created.out)

	before := call(rooted(root, "get", id, "--json")...)
	if before.err != nil {
		t.Fatalf("get returned error: %v (%s)", before.err, before.errOut)
	}

	// The flag is written after the identifier on purpose, so the test also
	// covers a flag that has to move in front of the positional arguments.
	updated := call(rooted(root, "update", id, "--content-from", bodyFile(t, "the new marker trinado lives here\n"))...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	var one, original service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &one); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	if err := json.Unmarshal([]byte(before.out), &original); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, before.out)
	}
	if one.Content != "the new marker trinado lives here\n" {
		t.Errorf("content = %q, want the file exactly", one.Content)
	}
	if one.ID != id || one.Title != "Kept memory" {
		t.Errorf("id or title changed: %q %q", one.ID, one.Title)
	}
	if strings.Join(one.Tags, ",") != "go,arch" {
		t.Errorf("tags = %v, want them kept", one.Tags)
	}
	if one.CreatedAt != original.CreatedAt {
		t.Errorf("created_at = %s, want it kept, %s", one.CreatedAt, original.CreatedAt)
	}
}

func TestUpdateFromAFileRefreshesTheIndex(t *testing.T) {
	root := t.TempDir()
	id := createMemory(t, root, "Moving memory", "the old marker zumbido lives here")

	from := bodyFile(t, "the new marker trinado lives here")
	updated := call(rooted(root, "update", id, "--content-from", from)...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	found := call(rooted(root, "search", "trinado")...)
	if found.err != nil {
		t.Fatalf("search returned error: %v (%s)", found.err, found.errOut)
	}
	if !strings.Contains(found.out, id) {
		t.Errorf("the word from the new body is not searchable:\n%s", found.out)
	}

	gone := call(rooted(root, "search", "zumbido")...)
	if gone.err != nil {
		t.Fatalf("search returned error: %v (%s)", gone.err, gone.errOut)
	}
	if strings.TrimSpace(gone.out) != "" {
		t.Errorf("the word from the old body is still searchable:\n%s", gone.out)
	}
}

func TestUpdateRejectsABodyFileItCannotUse(t *testing.T) {
	body := "body that must stay"

	cases := map[string]string{
		"missing":    filepath.Join(t.TempDir(), "nope.md"),
		"directory":  t.TempDir(),
		"empty file": bodyFile(t, ""),
	}

	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			id := createMemory(t, root, "Untouched memory", body)

			result := call(rooted(root, "update", id, "--content-from", path)...)
			if result.err == nil {
				t.Fatalf("update accepted a body file that is %s", name)
			}
			if !strings.Contains(result.err.Error(), path) {
				t.Errorf("error = %q, want it to name the path", result.err)
			}
			if got := storedBody(t, root, id); got != body {
				t.Errorf("body = %q, want it untouched, %q", got, body)
			}
		})
	}
}

func TestUpdateRejectsAFileAndTextTogether(t *testing.T) {
	root := t.TempDir()
	body := "body that must stay"
	id := createMemory(t, root, "Untouched memory", body)

	from := bodyFile(t, "replacement")
	result := call(rooted(root, "update", id, "--content-from", from, "and also this text")...)
	if result.err == nil {
		t.Fatal("update accepted a body from a file and from text")
	}
	if !strings.Contains(result.err.Error(), "--content-from") {
		t.Errorf("error = %q, want it to name the flag", result.err)
	}
	if got := storedBody(t, root, id); got != body {
		t.Errorf("body = %q, want it untouched, %q", got, body)
	}
}

func TestUpdateAppendsTextToTheBody(t *testing.T) {
	root := t.TempDir()
	created := call(rooted(root, "add", "--title", "Growing memory", "--tags", "go,arch",
		"the old marker zumbido lives here")...)
	if created.err != nil {
		t.Fatalf("add returned error: %v (%s)", created.err, created.errOut)
	}
	id := strings.TrimSpace(created.out)

	before := call(rooted(root, "get", id, "--json")...)
	if before.err != nil {
		t.Fatalf("get returned error: %v (%s)", before.err, before.errOut)
	}

	updated := call(rooted(root, "update", id, "--append", "the new marker trinado lands here")...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	shown := call(rooted(root, "get", id, "--json")...)
	if shown.err != nil {
		t.Fatalf("get returned error: %v (%s)", shown.err, shown.errOut)
	}
	var one, original service.MemoryJSON
	if err := json.Unmarshal([]byte(shown.out), &one); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, shown.out)
	}
	if err := json.Unmarshal([]byte(before.out), &original); err != nil {
		t.Fatalf("get did not print JSON: %v\n%s", err, before.out)
	}
	want := "the old marker zumbido lives here\nthe new marker trinado lands here"
	if one.Content != want {
		t.Errorf("content = %q, want %q", one.Content, want)
	}
	if one.ID != id || one.Title != "Growing memory" {
		t.Errorf("id or title changed: %q %q", one.ID, one.Title)
	}
	if strings.Join(one.Tags, ",") != "go,arch" {
		t.Errorf("tags = %v, want them kept", one.Tags)
	}
	if one.CreatedAt != original.CreatedAt {
		t.Errorf("created_at = %s, want it kept, %s", one.CreatedAt, original.CreatedAt)
	}
}

func TestUpdateAppendSeparatesWithOneNewline(t *testing.T) {
	cases := map[string]struct {
		old      string
		addition string
		want     string
	}{
		"body without a trailing newline": {
			old:      "first",
			addition: "second",
			want:     "first\nsecond",
		},
		"body with a trailing newline": {
			old:      "first\n",
			addition: "second",
			want:     "first\nsecond",
		},
		"addition keeping its own newline": {
			old:      "first",
			addition: "second\n",
			want:     "first\nsecond\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			id := createMemory(t, root, "Joined memory", tc.old)

			updated := call(rooted(root, "update", id, "--append", tc.addition)...)
			if updated.err != nil {
				t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
			}
			if got := storedBody(t, root, id); got != tc.want {
				t.Errorf("body = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUpdateAppendsFromAFile(t *testing.T) {
	root := t.TempDir()
	id := createMemory(t, root, "Filed memory", "kept body")

	from := bodyFile(t, "filed addition")
	updated := call(rooted(root, "update", id, "--append-from", from)...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}
	if got := storedBody(t, root, id); got != "kept body\nfiled addition" {
		t.Errorf("body = %q, want the file added to the kept body", got)
	}
}

func TestUpdateAppendRefreshesTheIndex(t *testing.T) {
	root := t.TempDir()
	id := createMemory(t, root, "Indexed memory", "the old marker zumbido lives here")

	updated := call(rooted(root, "update", id, "--append", "the new marker trinado lands here")...)
	if updated.err != nil {
		t.Fatalf("update returned error: %v (%s)", updated.err, updated.errOut)
	}

	found := call(rooted(root, "search", "trinado")...)
	if found.err != nil {
		t.Fatalf("search returned error: %v (%s)", found.err, found.errOut)
	}
	if !strings.Contains(found.out, id) {
		t.Errorf("the word from the addition is not searchable:\n%s", found.out)
	}

	kept := call(rooted(root, "search", "zumbido")...)
	if kept.err != nil {
		t.Fatalf("search returned error: %v (%s)", kept.err, kept.errOut)
	}
	if !strings.Contains(kept.out, id) {
		t.Errorf("the word from the kept body is no longer searchable:\n%s", kept.out)
	}
}

func TestUpdateRejectsAnEmptyAddition(t *testing.T) {
	body := "body that must stay"

	values := map[string]string{
		"empty flag value": "",
		"blank flag value": "   ",
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			id := createMemory(t, root, "Untouched memory", body)

			result := call(rooted(root, "update", id, "--append", value)...)
			if result.err == nil {
				t.Fatalf("update accepted an addition that is %s", name)
			}
			if !strings.Contains(result.err.Error(), "--append") {
				t.Errorf("error = %q, want it to name the flag", result.err)
			}
			if got := storedBody(t, root, id); got != body {
				t.Errorf("body = %q, want it untouched, %q", got, body)
			}
		})
	}

	t.Run("empty file", func(t *testing.T) {
		root := t.TempDir()
		id := createMemory(t, root, "Untouched memory", body)

		empty := bodyFile(t, "")
		result := call(rooted(root, "update", id, "--append-from", empty)...)
		if result.err == nil {
			t.Fatal("update accepted an addition from an empty file")
		}
		if !strings.Contains(result.err.Error(), empty) {
			t.Errorf("error = %q, want it to name the path", result.err)
		}
		if got := storedBody(t, root, id); got != body {
			t.Errorf("body = %q, want it untouched, %q", got, body)
		}
	})
}

func TestUpdateRejectsAnAdditionWithAReplacement(t *testing.T) {
	body := "body that must stay"
	replacement := bodyFile(t, "replacement")
	addition := bodyFile(t, "addition")

	cases := map[string][]string{
		"append with text":              {"--append", "extra", "and text"},
		"append from with text":         {"--append-from", addition, "and text"},
		"append with content from":      {"--append", "extra", "--content-from", replacement},
		"append from with content from": {"--append-from", addition, "--content-from", replacement},
		"append with append from":       {"--append", "extra", "--append-from", addition},
	}

	for name, rest := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			id := createMemory(t, root, "Untouched memory", body)

			result := call(append(rooted(root, "update", id), rest...)...)
			if result.err == nil {
				t.Fatalf("update accepted %s", name)
			}
			if got := storedBody(t, root, id); got != body {
				t.Errorf("body = %q, want it untouched, %q", got, body)
			}
		})
	}
}

func TestReindexRejectsPositionalArguments(t *testing.T) {
	root := t.TempDir()
	result := call(rooted(root, "reindex", "junk")...)
	if result.err == nil {
		t.Fatal("reindex accepted a positional argument")
	}
}
