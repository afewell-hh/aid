// Unit test — Algorithm 5 (mesh inter-switch link count).

test "Algorithm 5: mesh cable count" {
  // 2-switch mesh: all mesh-zone logical ports form the single pair's bundle.
  // mesh-two-switch fixture: 4 logical mesh ports per switch -> 4 cables.
  assert_eq(mesh_cable_count(2, 4), 4)
  assert_eq(mesh_cable_count(2, 32), 32)
  // 3-switch mesh (full mesh, 3 pairs): floor(ports / 2) per pair, must be even.
  assert_eq(mesh_cable_count(3, 8), 4)
  assert_eq(mesh_cable_count(3, 12), 6)
}
