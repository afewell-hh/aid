package planstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestCreate_DuplicateID_DoesNotOverwrite is the #88 defense-in-depth contract at
// the store layer: Store.Create of an id that already exists must NOT silently
// overwrite the existing plan.
//
// RED: today Create os.WriteFile's unconditionally (planstore.go), so the second
// create clobbers the first and returns a nil error. GREEN guards create — the
// preferred design is a distinct conflict error (a new sentinel, mapped to HTTP
// 409 at the API), leaving PUT/Update as the explicit overwrite path. This test
// asserts observable behavior (an error is returned AND the first plan survives)
// rather than a specific sentinel name, so it does not over-fit GREEN's choice;
// the API-layer test pins the 409 mapping.
func TestCreate_DuplicateID_DoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	first := []byte("id: dup\nname: First Plan\nstatus: draft\n")
	if _, err := s.Create(first); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// A second create of the SAME id with different content must be refused.
	second := []byte("id: dup\nname: Second Plan\nstatus: active\n")
	if _, err := s.Create(second); err == nil {
		t.Fatal("second Create of an existing id returned nil error — it silently overwrote the first plan (want a conflict error)")
	} else if errors.Is(err, ErrInvalidPlan) || errors.Is(err, ErrInvalidID) || errors.Is(err, ErrNotFound) {
		// A duplicate is a conflict — distinct from a malformed/absent/unsafe plan
		// (those map to 400/404). Reusing one of them would mis-signal the cause.
		t.Errorf("duplicate Create returned a mis-mapped sentinel %v; want a distinct conflict error", err)
	}

	// The first plan's summary must survive unchanged.
	got, err := s.Get("dup")
	if err != nil {
		t.Fatalf("Get after duplicate Create: %v", err)
	}
	if got.Name != "First Plan" || got.Status != "draft" {
		t.Errorf("duplicate Create overwrote the first plan: got name=%q status=%q, want First Plan/draft", got.Name, got.Status)
	}

	// And the on-disk bytes are byte-identical to the first plan.
	b, err := os.ReadFile(filepath.Join(dir, "dup.yaml"))
	if err != nil {
		t.Fatalf("read stored plan: %v", err)
	}
	if string(b) != string(first) {
		t.Errorf("stored bytes were overwritten:\n got %q\nwant %q", string(b), string(first))
	}
}

// TestUpdate_RemainsTheOverwritePath is a green-stays-green guard: PUT/Update is
// the EXPLICIT overwrite path and must keep working after the create-duplicate
// guard lands (#88 must not collaterally lock updates). Passes today; the point
// is that it keeps passing through GREEN.
func TestUpdate_RemainsTheOverwritePath(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create([]byte("id: dup\nname: First Plan\nstatus: draft\n")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Update("dup", []byte("id: dup\nname: Renamed\nstatus: active\n")); err != nil {
		t.Fatalf("Update of an existing plan must succeed: %v", err)
	}
	got, err := s.Get("dup")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Renamed" || got.Status != "active" {
		t.Errorf("Update did not overwrite: got name=%q status=%q, want Renamed/active", got.Name, got.Status)
	}
}
