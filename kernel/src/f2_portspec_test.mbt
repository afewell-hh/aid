// F2 RED — port-spec parsing fixtures (note §2.2; port_specification.py:39-77).
// These FAIL against the f2_calc.mbt stub (returns []) until GREEN.

test "F2 §2.2 parse_port_spec: plain range expands inclusive" {
  assert_eq(parse_port_spec("1-16"), [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16])
}

test "F2 §2.2 parse_port_spec: stepped range start-end:step" {
  // every 2nd port across 1-32
  assert_eq(parse_port_spec("1-32:2"), [1, 3, 5, 7, 9, 11, 13, 15, 17, 19, 21, 23, 25, 27, 29, 31])
}

test "F2 §2.2 parse_port_spec: comma-list, sorted + de-duplicated" {
  // real soc_storage 4x200 server zone
  assert_eq(parse_port_spec("27,29,31,33,35,37"), [27, 29, 31, 33, 35, 37])
}

test "F2 §2.2 parse_port_spec: mixed comma-list + range (real uplink zone shape)" {
  // a reduced slice of soc_storage uplink `26,28,30,32,34,36,38-63`
  assert_eq(parse_port_spec("26,28,38-41"), [26, 28, 38, 39, 40, 41])
  // duplicates collapse, output ascending regardless of input order
  assert_eq(parse_port_spec("5,1-3,2"), [1, 2, 3, 5])
}
