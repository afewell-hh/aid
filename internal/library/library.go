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
package library

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/afewell-hh/aid/internal/catalog"
	"github.com/afewell-hh/aid/internal/objectmodel"
	"github.com/afewell-hh/aid/internal/templates"
	"github.com/afewell-hh/aid/internal/topology"
)

// ErrConflict is returned when two source catalogs define the SAME pinned id with
// DIFFERENT content. Pinned identity must be reproducible (D21 guardrail 1), so a
// genuine conflict is a hard error, not a silent last-write-wins.
var ErrConflict = errors.New("library: conflicting definitions for same pinned id")

// Union merges catalogs, deduping items by pinned objectmodel.ID. Two items that
// share an id with identical content collapse to one; the same id with differing
// content is ErrConflict.
func Union(cats ...*catalog.Catalog) (*catalog.Catalog, error) {
	seen := make(map[objectmodel.ID]catalog.Item)
	for _, c := range cats {
		if c == nil {
			continue
		}
		for _, it := range c.Items() {
			if prev, dup := seen[it.ID]; dup {
				if !reflect.DeepEqual(prev, it) {
					return nil, fmt.Errorf("%w: %s", ErrConflict, it.ID)
				}
				continue
			}
			seen[it.ID] = it
		}
	}
	items := make([]catalog.Item, 0, len(seen))
	for _, it := range seen {
		items = append(items, it)
	}
	return catalog.New(items...)
}

// BuiltinCatalog returns the built-in Library: the deduped union of the catalogs
// derived from every shipped reference template (templates.IDs()). Each template's
// bundled training.yaml is ingested with the real engine ingest
// (topology.IngestBundled) and enriched with its optic/identity overlay (the same
// catalog.Merge design.Resolve uses) so Library rows carry real SKU identity.
func BuiltinCatalog() (*catalog.Catalog, error) {
	var cats []*catalog.Catalog
	for _, id := range templates.IDs() {
		training, ok := templates.Training(id)
		if !ok {
			return nil, fmt.Errorf("library: template %q has no training.yaml", id)
		}
		_, cat, err := topology.IngestBundled(training)
		if err != nil {
			return nil, fmt.Errorf("library: ingest template %q: %w", id, err)
		}
		if overlay, ok := templates.Overlay(id); ok && len(overlay) > 0 {
			ov, err := catalog.LoadBytes(overlay)
			if err != nil {
				return nil, fmt.Errorf("library: load overlay for %q: %w", id, err)
			}
			cat.Merge(ov)
		}
		cats = append(cats, cat)
	}
	return Union(cats...)
}
