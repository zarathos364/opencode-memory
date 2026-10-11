package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	var hits []memoryJSON
	if err := json.Unmarshal([]byte(result.out), &hits); err != nil {
		t.Fatalf("search did not print JSON: %v\n%s", err, result.out)
	}
	if len(hits) != 1 {
		t.Errorf("search returned %d hits, want 1", len(hits))
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
	createMemory(t, root, "Listed memory", "body")

	result := call(rooted(root, "list", "--json")...)
	if result.err != nil {
		t.Fatalf("list returned error: %v (%s)", result.err, result.errOut)
	}

	var listed []memoryJSON
	if err := json.Unmarshal([]byte(result.out), &listed); err != nil {
		t.Fatalf("list did not print JSON: %v\n%s", err, result.out)
	}
	if len(listed) != 1 {
		t.Errorf("list returned %d memories, want 1", len(listed))
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
