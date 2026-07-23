// Package planstore is the API's plan persistence layer: a directory of
// canonical plan YAML files (D9), one file per plan at <dir>/<id>.yaml. It is
// the Stage-A backing store for the REST plan-CRUD routes and the source of the
// plan YAML fed to internal/orchestrate for calc/bom/wiring. It is independent
// of internal/state (the inert IR-hash tracker, #36).
//
// Plan ids are traversal-sanitized: an id only ever names a single file inside
// the plans directory ([A-Za-z0-9_-]+), so "..", "/", and absolute paths are
// rejected before any filesystem access.
package planstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when no plan exists for the given id.
var ErrNotFound = errors.New("planstore: plan not found")

// ErrInvalidID is returned when an id is unsafe (path traversal, separators, or
// empty). The id only ever names a single file inside the plans directory.
var ErrInvalidID = errors.New("planstore: invalid plan id")

// ErrInvalidPlan is returned when plan YAML cannot be parsed or carries no
// usable id/name.
var ErrInvalidPlan = errors.New("planstore: invalid plan")

// ErrConflict is returned by Create when a plan with the derived id already
// exists. Create never overwrites — Update is the explicit overwrite path. The
// API maps this to HTTP 409 Conflict (#88 defense-in-depth).
var ErrConflict = errors.New("planstore: plan already exists")

// idPattern is the allowed id shape: it can only ever name a file inside the
// plans dir (no separators, no dot-segments).
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Plan is a stored plan. Summaries (List) carry id/name/status only; detail
// (Get) additionally carries the canonical YAML.
type Plan struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	YAML   string `json:"yaml,omitempty"`
}

// planMeta is the subset of plan YAML the store reads for summaries. Identity is
// canonically carried under the nested `meta:` block (meta.case_id / meta.name —
// the real DIET/XOC shape the engine ingests, see internal/topology.Meta); the
// top-level id/name fields are a fallback for legacy/non-DIET plans. parseMeta
// resolves the canonical fields into ID/Name/Status so callers read them flat.
type planMeta struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	Status string `yaml:"status"`
	Meta   struct {
		ID     string `yaml:"id"`
		CaseID string `yaml:"case_id"`
		Name   string `yaml:"name"`
		Status string `yaml:"status"`
	} `yaml:"meta"`
}

// Store is a plan store rooted at a directory of <id>.yaml files.
type Store struct {
	dir string
}

// Open opens (creating if needed) a plan store at dir.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// Dir returns the store's root directory.
func (s *Store) Dir() string { return s.dir }

// validID reports whether id is a safe single-file name inside the plans dir.
func validID(id string) bool { return idPattern.MatchString(id) }

// path returns the on-disk path for id, or ErrInvalidID if id is unsafe.
func (s *Store) path(id string) (string, error) {
	if !validID(id) {
		return "", ErrInvalidID
	}
	return filepath.Join(s.dir, id+".yaml"), nil
}

// List returns plan summaries (id/name/status, no YAML). Non-.yaml files and
// unsafe-named files are skipped.
func (s *Store) List() ([]Plan, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	plans := []Plan{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yaml")
		if !validID(id) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		meta, _ := parseMeta(b) // a summary tolerates an unparseable body
		plans = append(plans, Plan{ID: id, Name: meta.Name, Status: meta.Status})
	}
	return plans, nil
}

// Get returns the full plan (incl. YAML), ErrNotFound, or ErrInvalidID.
func (s *Store) Get(id string) (*Plan, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	meta, _ := parseMeta(b)
	return &Plan{ID: id, Name: meta.Name, Status: meta.Status, YAML: string(b)}, nil
}

// GetYAML returns the raw plan YAML bytes for id (fed to orchestrate),
// ErrNotFound, or ErrInvalidID.
func (s *Store) GetYAML(id string) ([]byte, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

// overlayPath returns the on-disk path for id's optic/identity overlay
// (<id>.overlay.yaml), or ErrInvalidID if id is unsafe. The overlay file shares
// the .yaml suffix but its trimmed name (`<id>.overlay`) carries a dot, which
// validID rejects — so List/Get never surface it as a phantom plan.
func (s *Store) overlayPath(id string) (string, error) {
	if !validID(id) {
		return "", ErrInvalidID
	}
	return filepath.Join(s.dir, id+".overlay.yaml"), nil
}

// GetOverlay returns id's overlay YAML verbatim, ErrNotFound if no overlay has
// been set, or ErrInvalidID. The F7 compute handlers treat ErrNotFound as "no
// overlay supplied" and proceed with the base catalog.
func (s *Store) GetOverlay(id string) ([]byte, error) {
	path, err := s.overlayPath(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

// SetOverlay stores id's overlay YAML verbatim (<id>.overlay.yaml). The plan must
// already exist (ErrNotFound otherwise); an unsafe id returns ErrInvalidID.
func (s *Store) SetOverlay(id string, overlayBytes []byte) error {
	planPath, err := s.path(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(planPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	opath, err := s.overlayPath(id)
	if err != nil {
		return err
	}
	return os.WriteFile(opath, overlayBytes, 0o644)
}

// Create parses id/name/status from the plan YAML (deriving the id from the name
// when absent), writes <id>.yaml, and returns the summary. A malformed plan
// returns ErrInvalidPlan; an unsafe id returns ErrInvalidID.
func (s *Store) Create(yamlBytes []byte) (*Plan, error) {
	meta, err := parseMeta(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	id := meta.ID
	if id == "" {
		id = slugify(meta.Name)
	}
	if id == "" {
		return nil, fmt.Errorf("%w: plan has no id or name", ErrInvalidPlan)
	}
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	// Create never overwrites: open with O_EXCL so an existing <id>.yaml is a
	// conflict, never a truncating write (#88). Update is the explicit overwrite
	// path. O_EXCL makes the existence check + create atomic (no TOCTOU window).
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrConflict, id)
		}
		return nil, err
	}
	if _, err := f.Write(yamlBytes); err != nil {
		f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &Plan{ID: id, Name: meta.Name, Status: meta.Status}, nil
}

// Update overwrites an existing plan's YAML, returning the summary, ErrNotFound,
// ErrInvalidID, or ErrInvalidPlan. The URL id is authoritative for the filename.
func (s *Store) Update(id string, yamlBytes []byte) (*Plan, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	meta, err := parseMeta(yamlBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPlan, err)
	}
	if err := os.WriteFile(path, yamlBytes, 0o644); err != nil {
		return nil, err
	}
	return &Plan{ID: id, Name: meta.Name, Status: meta.Status}, nil
}

// Delete removes a plan, returning ErrNotFound if absent or ErrInvalidID if id
// is unsafe.
func (s *Store) Delete(id string) error {
	path, err := s.path(id)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	// Best-effort: drop the companion overlay so it can't silently re-attach to a
	// future plan reusing this id.
	if opath, err := s.overlayPath(id); err == nil {
		_ = os.Remove(opath)
	}
	return nil
}

// parseMeta extracts id/name/status from plan YAML. A YAML syntax error is
// returned to the caller (mapped to ErrInvalidPlan on the write paths).
func parseMeta(b []byte) (planMeta, error) {
	var m planMeta
	if err := yaml.Unmarshal(b, &m); err != nil {
		return planMeta{}, err
	}
	// Canonical DIET identity lives under `meta:` (meta.case_id / meta.name).
	// Prefer it; fall back to any top-level id/name so non-DIET plans still work.
	if m.ID == "" {
		switch {
		case m.Meta.CaseID != "":
			m.ID = m.Meta.CaseID
		case m.Meta.ID != "":
			m.ID = m.Meta.ID
		}
	}
	if m.Name == "" && m.Meta.Name != "" {
		m.Name = m.Meta.Name
	}
	if m.Status == "" && m.Meta.Status != "" {
		m.Status = m.Meta.Status
	}
	return m, nil
}

// slugify reduces a name to a safe id ([a-z0-9-]). Returns "" if nothing usable.
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
