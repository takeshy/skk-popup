package dict

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStoreAt(filepath.Join(t.TempDir(), "skk-popup"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStorePersistsAcrossReload(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skk-popup")
	s, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.LoadUserDict(); got != "{}" {
		t.Fatalf("expected empty userdict, got %q", got)
	}
	if err := s.SaveUserDict(`{"みてい":["未定"]}`); err != nil {
		t.Fatal(err)
	}
	s.Flush()

	reloaded, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string][]string
	if err := json.Unmarshal([]byte(reloaded.LoadUserDict()), &decoded); err != nil {
		t.Fatal(err)
	}
	if want := []string{"未定"}; len(decoded["みてい"]) != 1 || decoded["みてい"][0] != want[0] {
		t.Fatalf("roundtrip mismatch: %#v", decoded)
	}
}

func TestStoreRejectsInvalidJSON(t *testing.T) {
	s := testStore(t)
	if err := s.SaveUserDict("{not json"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStorePersistsInputHistory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "skk-popup")
	s, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveInputHistory(`["first","second"]`); err != nil {
		t.Fatal(err)
	}
	s.Flush()

	reloaded, err := NewStoreAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.LoadInputHistory(); got != `["first","second"]` {
		t.Fatalf("input history roundtrip mismatch: %s", got)
	}
}

func TestWriteAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := writeAtomic(path, `{"a":1}`); err != nil {
		t.Fatal(err)
	}
	entries, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0] != path {
		t.Fatalf("temp files left behind: %v", entries)
	}
}
