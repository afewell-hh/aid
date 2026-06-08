// Package state holds minimal local state and config. State tracks the last IR
// hash per plan-file path so the CLI can flag "plan changed since last calc".
//
// Per the Phase-6 architecture note, the MVP uses a cgo-free flat-file JSON
// store under ~/.aid/ to preserve the single-static-binary goal (D4) without a
// cgo SQLite driver or a Go-toolchain bump; SQLite (modernc.org/sqlite) is the
// tracked upgrade. Config is ~/.aid/config.yaml (NetBox URL/token, default
// site/fabric) — loaded and surfaced, unused for compute this phase.
//
// RED phase: the store is unimplemented.
package state

import "errors"

// ErrNotImplemented is returned by the RED-phase stubs.
var ErrNotImplemented = errors.New("state: not implemented (RED)")

// Store is the local state store (flat-file JSON under ~/.aid/state.json).
type Store struct{}

// Open opens (or creates) the state store at the default location.
func Open() (*Store, error) { return nil, ErrNotImplemented }

// LastIRHash returns the recorded IR hash for a plan path, and whether one
// exists.
func (s *Store) LastIRHash(planPath string) (string, bool) { return "", false }

// SetIRHash records the IR hash for a plan path.
func (s *Store) SetIRHash(planPath, hash string) error { return ErrNotImplemented }

// Config is the user config loaded from ~/.aid/config.yaml.
type Config struct {
	NetBoxURL     string `yaml:"netbox_url"`
	NetBoxToken   string `yaml:"netbox_token"`
	DefaultSite   string `yaml:"default_site"`
	DefaultFabric string `yaml:"default_fabric"`
}

// LoadConfig reads ~/.aid/config.yaml (returns zero Config if absent).
func LoadConfig() (*Config, error) { return nil, ErrNotImplemented }
