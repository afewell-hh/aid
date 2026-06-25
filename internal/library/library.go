// Package library builds the read-only built-in Library surface (#80; spec #79
// "Ticket A") as the UNION of the catalogs derivable from the shipped built-in
// reference templates (internal/templates), deduped by pinned objectmodel.ID.
//
// Storage-minimalism (lead's #78 refinement, ratified in #79 revision 1): there
// is NO user-write store and NO new embedded catalog asset. The Library is
// derived at load time from the SAME embedded reference-template training.yaml
// bytes that back the reference gallery, via topology.IngestBundled. Coverage is
// therefore by construction: the Library is exactly the union of what the shipped
// references contain — both mesh (xoc-64) and Clos (xoc-256) classes — so it can
// never be narrower than the reference set (the lead's #79 blocking point).
//
// #80 builds the seam + RED stubs; the union/derivation lands in #80 GREEN.
package library

import (
	"errors"

	"github.com/afewell-hh/aid/internal/catalog"
)

// ErrNotImplemented marks an #80 RED stub whose behavior arrives in #80 GREEN.
var ErrNotImplemented = errors.New("library: not implemented (#80 GREEN)")

// ErrConflict is returned when two source catalogs define the SAME pinned id with
// DIFFERENT content. Pinned identity must be reproducible (D21 guardrail 1), so a
// genuine conflict is a hard error, not a silent last-write-wins.
var ErrConflict = errors.New("library: conflicting definitions for same pinned id")

// Union merges catalogs, deduping items by pinned objectmodel.ID. Two items that
// share an id with identical content collapse to one; the same id with differing
// content is ErrConflict.
func Union(cats ...*catalog.Catalog) (*catalog.Catalog, error) {
	return nil, ErrNotImplemented // RED stub (#80 GREEN)
}

// BuiltinCatalog returns the built-in Library: the deduped union of the catalogs
// derived from every shipped reference template (templates.IDs()). Built once.
func BuiltinCatalog() (*catalog.Catalog, error) {
	return nil, ErrNotImplemented // RED stub (#80 GREEN)
}
