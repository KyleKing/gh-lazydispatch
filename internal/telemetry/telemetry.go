// Package telemetry counts which features get invoked, in one local JSON file
// beside the frecency store. Nothing leaves the machine: there is no transport
// to configure, and the file holds event names and counts, never arguments.
package telemetry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
)

const (
	dirPerm  = 0o750
	filePerm = 0o600
)

// Event is how often one feature was invoked, and when.
type Event struct {
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
	Count     int       `json:"count"`
}

// Store is every event ever tracked, by name.
type Store struct {
	Events map[string]Event `json:"events"`
}

// CachePath is the telemetry file, in the directory the frecency store uses.
func CachePath() string {
	return filepath.Join(filepath.Dir(frecency.CachePath()), "telemetry.json")
}

// Load reads the store at path, returning an empty one when the file is absent.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path derived from the XDG cache directory
	if errors.Is(err, os.ErrNotExist) {
		return &Store{Events: make(map[string]Event)}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("reading telemetry %s: %w", path, err)
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parsing telemetry %s: %w", path, err)
	}

	if store.Events == nil {
		store.Events = make(map[string]Event)
	}

	return &store, nil
}

// Tracker counts events and writes each count through to its file. A nil
// Tracker tracks nothing, so tests and callers without a cache directory pass
// nil rather than a stub.
type Tracker struct {
	store *Store
	path  string
	mu    sync.Mutex
}

// Open loads the store at path and returns a tracker that writes to it.
func Open(path string) (*Tracker, error) {
	store, err := Load(path)
	if err != nil {
		return nil, err
	}

	return &Tracker{store: store, path: path}, nil
}

// Track counts one invocation of event. A failed write is dropped, because a
// usage counter must never get in the way of the action it counts.
func (t *Tracker) Track(event string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()

	e := t.store.Events[event]
	if e.Count == 0 {
		e.FirstSeen = now
	}

	e.Count++
	e.LastSeen = now
	t.store.Events[event] = e

	//nolint:errcheck // best-effort counter; see Track
	_ = t.save()
}

func (t *Tracker) save() error {
	if err := os.MkdirAll(filepath.Dir(t.path), dirPerm); err != nil {
		return fmt.Errorf("creating directory for telemetry %s: %w", t.path, err)
	}

	data, err := json.MarshalIndent(t.store, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling telemetry: %w", err)
	}

	tmp := t.path + ".tmp"
	if err := os.WriteFile(tmp, data, filePerm); err != nil {
		return fmt.Errorf("writing telemetry %s: %w", tmp, err)
	}

	if err := os.Rename(tmp, t.path); err != nil {
		return fmt.Errorf("replacing telemetry %s: %w", t.path, err)
	}

	return nil
}
