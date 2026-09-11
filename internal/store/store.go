// Package store persists collected pull requests as JSON. The dataset is small
// -- a few thousand records a year -- so a single file keeps the binary free of
// cgo and the tool free of a database.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/peterulsteen/shipped/internal/gh"
)

// Store holds every PR seen, keyed by ID, on both the authored and reviewed sides.
type Store struct {
	Authored map[string]gh.PullRequest `json:"authored"`
	Reviewed map[string]gh.PullRequest `json:"reviewed"`
	LastRun  *time.Time                `json:"last_run,omitempty"`
	Login    string                    `json:"login,omitempty"`
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		Authored: map[string]gh.PullRequest{},
		Reviewed: map[string]gh.PullRequest{},
	}
}

// Load reads the store, returning an empty one if the file does not exist yet.
func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	s := New()
	if err := json.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("%s is corrupt: %w", path, err)
	}
	if s.Authored == nil {
		s.Authored = map[string]gh.PullRequest{}
	}
	if s.Reviewed == nil {
		s.Reviewed = map[string]gh.PullRequest{}
	}
	return s, nil
}

// Save writes the store atomically, so an interrupted run cannot truncate it.
func (s *Store) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Merge upserts records, so a re-fetched PR picks up a merge that happened since.
func (s *Store) Merge(role gh.Role, prs []gh.PullRequest) int {
	target := s.Authored
	if role == gh.Reviewed {
		target = s.Reviewed
	}
	for _, pr := range prs {
		target[pr.ID()] = pr
	}
	return len(prs)
}
