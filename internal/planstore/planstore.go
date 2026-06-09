// Package planstore is the API's plan persistence layer: a directory of
// canonical plan YAML files (D9), one file per plan at <dir>/<id>.yaml. It is
// the Stage-A backing store for the REST plan-CRUD routes and the source of the
// plan YAML fed to internal/orchestrate for calc/bom/wiring. It is independent
// of internal/state (the inert IR-hash tracker, #36).
//
// RED phase (issue #11, Stage A): Open is real (it only records/creates the
// directory so handlers and tests can construct a store); every data operation
// is a stub returning ErrNotImplemented. GREEN implements them.
package planstore

import (
	"errors"
	"os"
)

// ErrNotImplemented is returned by the RED-phase data-operation stubs.
var ErrNotImplemented = errors.New("planstore: not implemented (Stage A GREEN)")

// ErrNotFound is returned when no plan exists for the given id.
var ErrNotFound = errors.New("planstore: plan not found")

// ErrInvalidID is returned when an id is unsafe (path traversal, separators, or
// empty). The id only ever names a single file inside the plans directory.
var ErrInvalidID = errors.New("planstore: invalid plan id")

// Plan is a stored plan. Summaries (List) carry id/name/status only; detail
// (Get) additionally carries the canonical YAML.
type Plan struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	YAML   string `json:"yaml,omitempty"`
}

// Store is a plan store rooted at a directory of <id>.yaml files.
type Store struct {
	dir string
}

// Open opens (creating if needed) a plan store at dir. This is real plumbing —
// it only ensures the directory exists; the data operations are the RED stubs.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the store's root directory.
func (s *Store) Dir() string { return s.dir }

// List returns plan summaries (id/name/status, no YAML).
func (s *Store) List() ([]Plan, error) { return nil, ErrNotImplemented }

// Get returns the full plan (incl. YAML), or ErrNotFound. An unsafe id returns
// ErrInvalidID.
func (s *Store) Get(id string) (*Plan, error) { return nil, ErrNotImplemented }

// GetYAML returns the raw plan YAML bytes for id (fed to orchestrate), or
// ErrNotFound. An unsafe id returns ErrInvalidID.
func (s *Store) GetYAML(id string) ([]byte, error) { return nil, ErrNotImplemented }

// Create parses id/name/status from the plan YAML, writes <id>.yaml, and returns
// the summary. A malformed plan returns an error; an unsafe/duplicate id returns
// ErrInvalidID / an error.
func (s *Store) Create(yamlBytes []byte) (*Plan, error) { return nil, ErrNotImplemented }

// Update overwrites an existing plan's YAML, returning the summary or
// ErrNotFound. An unsafe id returns ErrInvalidID.
func (s *Store) Update(id string, yamlBytes []byte) (*Plan, error) {
	return nil, ErrNotImplemented
}

// Delete removes a plan, returning ErrNotFound if absent. An unsafe id returns
// ErrInvalidID.
func (s *Store) Delete(id string) error { return ErrNotImplemented }
