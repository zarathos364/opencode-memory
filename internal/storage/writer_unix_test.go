//go:build unix

package storage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// The umask must reach the file, proving the writer does not override it: the
// base request is 0666, the same base touch requests, and the umask decides.
func TestCreateFollowsTheUmask(t *testing.T) {
	cases := map[string]struct {
		mask int
		want os.FileMode
	}{
		"umask 000 keeps the conventional base":  {mask: 0o000, want: 0o666},
		"umask 002 clears other write":           {mask: 0o002, want: 0o664},
		"umask 022 clears group and other write": {mask: 0o022, want: 0o644},
		"umask 077 keeps the file owner only":    {mask: 0o077, want: 0o600},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			previous := syscall.Umask(tc.mask)
			t.Cleanup(func() { syscall.Umask(previous) })

			raw := filepath.Join(t.TempDir(), "raw")
			w := NewWriter(raw)

			created, err := w.Create("title", "content", nil, false)
			if err != nil {
				t.Fatalf("Create returned error: %v", err)
			}

			info, err := os.Stat(filepath.Join(raw, created.ID+".md"))
			if err != nil {
				t.Fatalf("stat returned error: %v", err)
			}
			if got := info.Mode().Perm(); got != tc.want {
				t.Errorf("mode = %o under umask %03o, want %o", got, tc.mask, tc.want)
			}
		})
	}
}

func TestSensitiveFileStaysOwnerOnlyUnderAPermissiveUmask(t *testing.T) {
	previous := syscall.Umask(0o000)
	t.Cleanup(func() { syscall.Umask(previous) })

	raw := filepath.Join(t.TempDir(), "raw")
	w := NewWriter(raw)

	created, err := w.Create("title", "content", nil, true)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	info, err := os.Stat(filepath.Join(raw, created.ID+".md"))
	if err != nil {
		t.Fatalf("stat returned error: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o under umask 000, want 0600", got)
	}
}
