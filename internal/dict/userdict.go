// Package dict persists the user dictionary and candidate history as
// JSON files under the per-user data directory ($XDG_DATA_HOME/skk-popup on
// Linux; see dataDir for Windows/macOS).
//
// Writes are debounced (flushed 2 seconds after the last update) and are
// always flushed when the popup hides. All writes go through a temporary
// file + rename so a crash never leaves truncated files behind.
package dict

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	UserDictFile     = "userdict.json"
	HistoryFile      = "history.json"
	InputHistoryFile = "input-history.json"
	flushInterval    = 2 * time.Second
)

type Store struct {
	mu                sync.Mutex
	dir               string
	userDictJSON      string
	historyJSON       string
	inputHistoryJSON  string
	userDictDirty     bool
	historyDirty      bool
	inputHistoryDirty bool
	timer             *time.Timer
}

// dataDir returns the per-user data directory:
// $XDG_DATA_HOME/skk-popup (or ~/.local/share/skk-popup) on Linux,
// %LocalAppData%/skk-popup on Windows, and
// ~/Library/Application Support/skk-popup on macOS.
func dataDir() string {
	if base := os.Getenv("XDG_DATA_HOME"); base != "" {
		return filepath.Join(base, "skk-popup")
	}
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "skk-popup")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "skk-popup")
		}
	default:
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".local", "share", "skk-popup")
		}
	}
	return ""
}

// DataDir returns the per-user data directory (see dataDir).
func DataDir() string { return dataDir() }

// NewStore creates a store rooted at $XDG_DATA_HOME/skk-popup.
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

// LoadInputHistory returns the persisted clipboard input history JSON.
func (s *Store) LoadInputHistory() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inputHistoryJSON == "" {
		s.inputHistoryJSON = readFileOrEmpty(filepath.Join(s.dir, InputHistoryFile), "[]")
	}
	return s.inputHistoryJSON
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

// SaveInputHistory stages a clipboard input history update.
func (s *Store) SaveInputHistory(data string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !json.Valid([]byte(data)) {
		return os.ErrInvalid
	}
	s.inputHistoryJSON = data
	s.inputHistoryDirty = true
	s.scheduleFlushLocked()
	return nil
}

// Flush writes any staged updates to disk immediately. Failed writes remain
// dirty and are retried after the debounce interval.
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	var flushErrors []error
	if s.userDictDirty {
		if err := writeAtomic(filepath.Join(s.dir, UserDictFile), s.userDictJSON); err == nil {
			s.userDictDirty = false
		} else {
			flushErrors = append(flushErrors, fmt.Errorf("%s: %w", UserDictFile, err))
		}
	}
	if s.historyDirty {
		if err := writeAtomic(filepath.Join(s.dir, HistoryFile), s.historyJSON); err == nil {
			s.historyDirty = false
		} else {
			flushErrors = append(flushErrors, fmt.Errorf("%s: %w", HistoryFile, err))
		}
	}
	if s.inputHistoryDirty {
		if err := writeAtomic(filepath.Join(s.dir, InputHistoryFile), s.inputHistoryJSON); err == nil {
			s.inputHistoryDirty = false
		} else {
			flushErrors = append(flushErrors, fmt.Errorf("%s: %w", InputHistoryFile, err))
		}
	}
	if len(flushErrors) > 0 {
		s.scheduleFlushLocked()
	}
	return errors.Join(flushErrors...)
}

func (s *Store) scheduleFlushLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushInterval, func() {
		if err := s.Flush(); err != nil {
			log.Printf("dict store flush: %v", err)
		}
	})
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
	tmp, err := os.CreateTemp(dir, ".skk-popup-*.tmp")
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
