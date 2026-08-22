// Package dict persists the user dictionary and candidate history as
// JSON files under the per-user data directory ($XDG_DATA_HOME/wl-skk on
// Linux; see dataDir for Windows/macOS).
//
// Writes are debounced (flushed 2 seconds after the last update) and are
// always flushed when the popup hides. All writes go through a temporary
// file + rename so a crash never leaves truncated files behind.
package dict

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	UserDictFile  = "userdict.json"
	HistoryFile   = "history.json"
	flushInterval = 2 * time.Second
)

type Store struct {
	mu            sync.Mutex
	dir           string
	userDictJSON  string
	historyJSON   string
	userDictDirty bool
	historyDirty  bool
	timer         *time.Timer
}

// dataDir returns the per-user data directory:
// $XDG_DATA_HOME/wl-skk (or ~/.local/share/wl-skk) on Linux,
// %LocalAppData%/wl-skk on Windows, and
// ~/Library/Application Support/wl-skk on macOS.
func dataDir() string {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "wl-skk")
	}
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "wl-skk")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "wl-skk")
		}
	default:
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".local", "share", "wl-skk")
		}
	}
	return ""
}

// NewStore creates a store rooted at $XDG_DATA_HOME/wl-skk.
func NewStore() (*Store, error) {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// NewStoreAt creates a store rooted at an explicit directory (tests).
func NewStoreAt(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// LoadUserDict returns the persisted user dictionary JSON ("{}" when absent).
func (s *Store) LoadUserDict() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.userDictJSON == "" {
		s.userDictJSON = readFileOrEmpty(filepath.Join(s.dir, UserDictFile), "{}")
	}
	return s.userDictJSON
}

// LoadHistory returns the persisted candidate history JSON ("{}" when absent).
func (s *Store) LoadHistory() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.historyJSON == "" {
		s.historyJSON = readFileOrEmpty(filepath.Join(s.dir, HistoryFile), "{}")
	}
	return s.historyJSON
}

// SaveUserDict stages a user dictionary update; it is flushed after the
// debounce interval.
func (s *Store) SaveUserDict(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !json.Valid([]byte(data)) {
		return os.ErrInvalid
	}
	s.userDictJSON = data
	s.userDictDirty = true
	s.scheduleFlushLocked()
	return nil
}

// SaveHistory stages a history update; it is flushed after the debounce interval.
func (s *Store) SaveHistory(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !json.Valid([]byte(data)) {
		return os.ErrInvalid
	}
	s.historyJSON = data
	s.historyDirty = true
	s.scheduleFlushLocked()
	return nil
}

// Flush writes any staged updates to disk immediately.
func (s *Store) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	if s.userDictDirty {
		if writeAtomic(filepath.Join(s.dir, UserDictFile), s.userDictJSON) == nil {
			s.userDictDirty = false
		}
	}
	if s.historyDirty {
		if writeAtomic(filepath.Join(s.dir, HistoryFile), s.historyJSON) == nil {
			s.historyDirty = false
		}
	}
}

func (s *Store) scheduleFlushLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushInterval, s.Flush)
}

func readFileOrEmpty(path, emptyValue string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return emptyValue
	}
	return string(data)
}

func writeAtomic(path, content string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".wl-skk-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
