// Stable, identifiable validation codes (DECISIONS.md D11; issue #6 condition 4).
// Invalid plans surface these as `ValidationResult.errors[].code` so Phases 5/6
// can react programmatically rather than parsing prose messages.

pub let code_mesh_switch_count : String = "MESH_SWITCH_COUNT"

pub let code_mclag_switch_count : String = "MCLAG_SWITCH_COUNT"

pub let code_eslag_switch_count : String = "ESLAG_SWITCH_COUNT"

pub let code_zone_capacity_deficit : String = "ZONE_CAPACITY_DEFICIT"

pub let code_mixed_distribution : String = "MIXED_DISTRIBUTION"

// Warning (non-blocking, D11).
pub let code_oversubscription : String = "OVERSUBSCRIPTION"
