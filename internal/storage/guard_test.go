package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/keshon/beacon/internal/storage"
)

// An old data directory must stop the server, not come up empty. Coming up
// empty is the dangerous outcome: the monitor reports nothing wrong because it
// is watching nothing.
func TestUnconvertedDataDirIsRefused(t *testing.T) {
	dir := t.TempDir()
	old := `{"monitors":{"m1":{"id":"m1","name":"legacy","type":"tcp","target":"example.com:80","enabled":true}}}`
	if err := os.WriteFile(filepath.Join(dir, "monitors.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	st, err := storage.New(dir)
	if err == nil {
		st.Close()
		t.Fatal("an unconverted directory started silently")
	}
	if !strings.Contains(err.Error(), "monitors.json") {
		t.Fatalf("the refusal does not name what it found: %v", err)
	}
}

// A fresh directory is not an old one, and must start.
func TestEmptyDataDirStarts(t *testing.T) {
	st, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("a fresh data dir was refused: %v", err)
	}
	st.Close()
}

// A converted directory that still has the old files beside it must start:
// the import already happened, and the leftovers are only clutter.
func TestConvertedDirWithLeftoversStarts(t *testing.T) {
	dir := t.TempDir()
	st, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	st.Close() // db/ now exists

	old := `{"monitors":{"m1":{"id":"m1","name":"legacy","type":"tcp","target":"example.com:80"}}}`
	if err := os.WriteFile(filepath.Join(dir, "monitors.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	again, err := storage.New(dir)
	if err != nil {
		t.Fatalf("a converted dir with leftovers was refused: %v", err)
	}
	again.Close()
}
