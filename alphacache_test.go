package contentpatcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Rethunk-Tech/go-safefs"
)

// Masks unused for a month are deleted on prune; a younger file stays.
func TestAlphaDiskPrunesMasksOlderThanKeepWindow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	oldPath := filepath.Join(dir, "old.gz")
	freshPath := filepath.Join(dir, "fresh.gz")
	writeAt(t, oldPath, []byte("old"))
	writeAt(t, freshPath, []byte("fresh"))
	old := time.Now().Add(-alphaKeepFor - 24*time.Hour)
	if err := os.Chtimes(oldPath, old, old); err != nil {
		t.Fatal(err)
	}
	pruneAlphaDir(dir)
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old mask still present: %v", err)
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh mask removed: %v", err)
	}
}

// Reading a mask idle longer than a day refreshes its mtime so prune does not drop it.
func TestAlphaDiskTouchesAMaskIdleLongerThanADay(t *testing.T) {
	t.Parallel()
	old, err := safefs.ReadFile("testdata/mask-gzip-19x5.gz")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "idle.gz")
	if err := safefs.WriteFile(path, old, 0o600); err != nil {
		t.Fatal(err)
	}
	idle := time.Now().Add(-alphaTouchAge - time.Hour)
	if err := os.Chtimes(path, idle, idle); err != nil {
		t.Fatal(err)
	}
	if readAlphaDisk(path) == nil {
		t.Fatal("idle mask did not read")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(info.ModTime()) > time.Minute {
		t.Fatalf("mtime still %v old after a read", time.Since(info.ModTime()))
	}
}

// A cache path whose parent cannot be created is skipped, so a nil mask is never written.
func TestAlphaDiskWriteIsSkippedWhenTheCacheDirCannotBeCreated(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	blocker := filepath.Join(parent, "not-a-dir")
	writeAt(t, blocker, []byte("x"))
	writeAlphaDisk(filepath.Join(blocker, "mask.gz"), nil)
	if entries, err := os.ReadDir(parent); err != nil || len(entries) != 1 {
		t.Fatalf("write created extra entries: %v %v", entries, err)
	}
}
