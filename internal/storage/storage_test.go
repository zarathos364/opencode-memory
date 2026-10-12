package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"
)

func newStore(t *testing.T) (*Writer, *Reader) {
	t.Helper()
	raw := filepath.Join(t.TempDir(), "raw")
	return NewWriter(raw), NewReader(raw)
}

func TestCreateRoundTrip(t *testing.T) {
	w, r := newStore(t)

	created, err := w.Create("Short Title", "body text", []string{"Go", "go", " BETA "}, true)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	got, err := r.Get(created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created.CreatedAt)
	}
	if got.Title != created.Title {
		t.Errorf("Title = %q, want %q", got.Title, created.Title)
	}
	if got.Content != created.Content {
		t.Errorf("Content = %q, want %q", got.Content, created.Content)
	}
	if !got.Sensitive {
		t.Error("Sensitive = false, want true")
	}
	if got.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, SchemaVersion)
	}
}

func TestCreatedFileIsHumanReadable(t *testing.T) {
	w, r := newStore(t)

	created, err := w.Create("Short Title", "body text", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(r.rawDir, created.ID+".md"))
	if err != nil {
		t.Fatalf("reading the stored file failed: %v", err)
	}
	file := string(body)

	for _, want := range []string{
		"---\n",
		"schema_version: 1",
		"title: Short Title",
		"tags: []",
		"sensitive: false",
		"\nbody text\n",
	} {
		if !strings.Contains(file, want) {
			t.Errorf("stored file is missing %q\ngot:\n%s", want, file)
		}
	}
	if !strings.HasPrefix(file, "---\n") {
		t.Errorf("stored file must open with the header marker, got:\n%s", file)
	}
}

func TestSensitiveFileIsReadableOnlyByItsOwner(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "raw")
	w := NewWriter(raw)

	plain, err := w.Create("title", "content", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	secret, err := w.Create("title", "content", nil, true)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// The test cannot read the umask without changing it, so a regular memory
	// is compared with a control file created the same way.
	control := filepath.Join(t.TempDir(), "control")
	file, err := os.OpenFile(control, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		t.Fatalf("creating the control file failed: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("closing the control file failed: %v", err)
	}

	plainInfo, err := os.Stat(filepath.Join(raw, plain.ID+".md"))
	if err != nil {
		t.Fatalf("stat returned error: %v", err)
	}
	controlInfo, err := os.Stat(control)
	if err != nil {
		t.Fatalf("stat returned error: %v", err)
	}
	if got, want := plainInfo.Mode().Perm(), controlInfo.Mode().Perm(); got != want {
		t.Errorf("mode = %o, want the umask-applied %o", got, want)
	}

	secretInfo, err := os.Stat(filepath.Join(raw, secret.ID+".md"))
	if err != nil {
		t.Fatalf("stat returned error: %v", err)
	}
	if got := secretInfo.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("sensitive mode = %o, want no group or other bits", got)
	}
}

func TestIdentifiersFollowCreationOrder(t *testing.T) {
	w, _ := newStore(t)

	const count = 5
	ids := make([]string, 0, count)
	var previous time.Time
	for i := 0; i < count; i++ {
		m, err := w.Create("title", "content", nil, false)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		if i > 0 {
			if m.CreatedAt.Before(previous) {
				t.Fatalf("CreatedAt %v went backwards from %v", m.CreatedAt, previous)
			}
			if strings.Compare(ids[i-1], m.ID) >= 0 {
				t.Fatalf("identifier %q does not sort after %q", m.ID, ids[i-1])
			}
		}
		previous = m.CreatedAt
		ids = append(ids, m.ID)
	}
}

func TestGetAcceptsSuffixAndRejectsUnsafeInput(t *testing.T) {
	w, r := newStore(t)
	created, err := w.Create("title", "content", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := r.Get(created.ID + ".md"); err != nil {
		t.Errorf("Get with suffix returned error: %v", err)
	}

	for _, id := range []string{"../../etc/passwd", "..", "not-a-uuid", "", "  "} {
		if _, err := r.Get(id); err == nil {
			t.Errorf("Get(%q) returned nil error, want rejection", id)
		}
	}

	if _, err := r.Get("00000000-0000-0000-0000-000000000000"); err == nil {
		t.Error("Get for a well formed but missing id returned nil error, want not found")
	}
}

func TestListIsEmptyBeforeAnythingIsWritten(t *testing.T) {
	r := NewReader(filepath.Join(t.TempDir(), "raw"))

	list, err := r.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List returned %d memories, want 0", len(list))
	}
}

func TestListReturnsEveryMemoryInCreationOrder(t *testing.T) {
	w, r := newStore(t)

	const count = 4
	want := make([]string, 0, count)
	for i := 0; i < count; i++ {
		m, err := w.Create("title", "content", nil, false)
		if err != nil {
			t.Fatalf("Create returned error: %v", err)
		}
		want = append(want, m.ID)
	}
	if _, err := w.Create("ignored", "content", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	list, err := r.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != count+1 {
		t.Fatalf("List returned %d memories, want %d", len(list), count+1)
	}
	for i, id := range want {
		if list[i].ID != id {
			t.Errorf("position %d = %q, want %q", i, list[i].ID, id)
		}
	}
}

func TestBodyKeepsItsTrailingNewlines(t *testing.T) {
	w, r := newStore(t)

	created, err := w.Create("title", "line one\nline two\n\n", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	got, err := r.Get(created.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if got.Content != created.Content {
		t.Errorf("Content = %q, want %q", got.Content, created.Content)
	}
}

func TestCreateNormalizesTags(t *testing.T) {
	w, _ := newStore(t)

	m, err := w.Create("title", "content", []string{" Go ", "go", "BETA", "", "  "}, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if len(m.Tags) != 2 {
		t.Fatalf("Tags = %v, want 2 entries", m.Tags)
	}
	if m.Tags[0] != "go" || m.Tags[1] != "beta" {
		t.Errorf("Tags = %v, want [go beta]", m.Tags)
	}
}

func TestCreateLeavesNoTemporaryFiles(t *testing.T) {
	w, _ := newStore(t)
	raw := w.rawDir

	if _, err := w.Create("title", "content", nil, false); err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	entries, err := os.ReadDir(raw)
	if err != nil {
		t.Fatalf("reading the raw directory failed: %v", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".md") {
			t.Errorf("unexpected file left behind: %q", entry.Name())
		}
	}
}

func TestValidateRejectsIncompleteMemories(t *testing.T) {
	base := func() *Memory {
		return &Memory{
			SchemaVersion: SchemaVersion,
			ID:            "0192f0c1-0000-7000-8000-000000000000",
			CreatedAt:     time.Now().UTC(),
			Tags:          []string{},
			Title:         "title",
			Content:       "content",
		}
	}

	cases := []struct {
		name   string
		memory *Memory
		change func(*Memory)
	}{
		{name: "nil memory", memory: nil},
		{name: "unknown schema version", memory: base(), change: func(m *Memory) { m.SchemaVersion = 99 }},
		{name: "missing identifier", memory: base(), change: func(m *Memory) { m.ID = "" }},
		{name: "path in identifier", memory: base(), change: func(m *Memory) { m.ID = "../../etc/passwd" }},
		{name: "missing creation time", memory: base(), change: func(m *Memory) { m.CreatedAt = time.Time{} }},
		{name: "empty title", memory: base(), change: func(m *Memory) { m.Title = "   " }},
		{name: "title spans lines", memory: base(), change: func(m *Memory) { m.Title = "line one\nline two" }},
		{name: "empty content", memory: base(), change: func(m *Memory) { m.Content = " " }},
		{name: "empty tag", memory: base(), change: func(m *Memory) { m.Tags = []string{"go", " "} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.change != nil {
				tc.change(tc.memory)
			}
			if err := tc.memory.Validate(); err == nil {
				t.Error("Validate returned nil error, want rejection")
			}
		})
	}
}

func TestParseRejectsBrokenFiles(t *testing.T) {
	cases := []string{
		"no header marker at all",
		"---\nunclosed header",
		"---\nschema_version: 1\n",
	}
	for _, file := range cases {
		if _, err := parseFile(file); err == nil {
			t.Errorf("parseFile(%q) returned nil error, want rejection", file)
		}
	}
}

func TestListSkipsFilesItCannotUse(t *testing.T) {
	raw := filepath.Join(t.TempDir(), "raw")
	w, r := NewWriter(raw), NewReader(raw)

	created, err := w.Create("title", "content", nil, false)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(raw, "notes.md"), []byte("not a memory"), 0o644); err != nil {
		t.Fatalf("write the stray file: %v", err)
	}
	broken := uuid.NewV7().String()
	if err := os.WriteFile(filepath.Join(raw, broken+".md"), []byte("---\nschema_version: 1\n"), 0o644); err != nil {
		t.Fatalf("write the broken file: %v", err)
	}

	list, err := r.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List returned %d memories, want 1", len(list))
	}
	if list[0].ID != created.ID {
		t.Errorf("List returned id %q, want %q", list[0].ID, created.ID)
	}

	if _, err := r.Get(broken); err == nil {
		t.Error("Get accepted a file it cannot parse, want the failure reported")
	}
}
