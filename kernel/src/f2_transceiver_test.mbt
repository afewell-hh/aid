// F2 RED — transceiver verdict fixtures (note §1.1, §2.5; transceiver_rules.py:160-256).
// Pins the catalog-free verdict contract: Go resolves the optic attribute_data,
// the kernel verdicts on (medium, cage_type, connector). FAIL against the
// f2_calc.mbt stub (returns ("","")) until GREEN.

test "F2 §2.5 medium mismatch BLOCKS (never downgraded)" {
  let s : XcvrAttrs = { medium: "SMF", cage_type: "OSFP", connector: "MPO-12" }
  let z : XcvrAttrs = { medium: "MMF", cage_type: "OSFP", connector: "MPO-12" }
  assert_eq(f2_xcvr_verdict(Some(s), Some(z)), ("blocked", "R_MEDIUM_MISMATCH"))
}

test "F2 §2.5 cage_type mismatch (same medium) -> needs_review" {
  let s : XcvrAttrs = { medium: "MMF", cage_type: "QSFP112", connector: "MPO-12" }
  let z : XcvrAttrs = { medium: "MMF", cage_type: "OSFP", connector: "MPO-12" }
  assert_eq(f2_xcvr_verdict(Some(s), Some(z)), ("needs_review", "R_CAGE_MISMATCH"))
}

test "F2 §2.5 both ends carry no optic -> match (R_NULL)" {
  assert_eq(f2_xcvr_verdict(None, None), ("match", "R_NULL"))
}

test "F2 §2.5 fully compatible optic -> match" {
  let s : XcvrAttrs = { medium: "MMF", cage_type: "OSFP", connector: "MPO-12" }
  assert_eq(f2_xcvr_verdict(Some(s), Some(s)), ("match", "R_MATCH"))
}
