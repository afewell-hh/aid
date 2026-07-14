// Shared JSON primitives for the live F2/F3 kernel boundary (DECISIONS.md D16/D28).
//
// These decode/encode helpers are the small, live core the F2 (`f2_calc.mbt`) and
// F3 (`f3_bom.mbt`) JSON codecs build on. They were extracted verbatim from the
// retired legacy `decode.mbt` / `encode.mbt` when the pre-rebuild WIT/topology-ir
// cluster was retired (#85, Option A), so the live boundary keeps exactly the
// primitives it already used with no wire change. Everything here is pure string /
// Json manipulation — no I/O, no externs.

priv suberror DecodeErr {
  DecodeErr(String)
}

// ---- decode primitives (raise on shape mismatch) --------------------------

fn d_obj(j : Json) -> Map[String, Json] raise DecodeErr {
  if j is Object(m) {
    m
  } else {
    raise DecodeErr("expected object")
  }
}

fn d_field(m : Map[String, Json], k : String) -> Json raise DecodeErr {
  match m.get(k) {
    Some(v) => v
    None => raise DecodeErr("missing field: " + k)
  }
}

fn d_str(j : Json) -> String raise DecodeErr {
  if j is String(s) {
    s
  } else {
    raise DecodeErr("expected string")
  }
}

fn d_arr(j : Json) -> Array[Json] raise DecodeErr {
  if j is Array(a) {
    a
  } else {
    raise DecodeErr("expected array")
  }
}

fn d_field_str(m : Map[String, Json], k : String) -> String raise DecodeErr {
  d_str(d_field(m, k))
}

fn d_arr_or_empty(m : Map[String, Json], k : String) -> Array[Json] raise DecodeErr {
  match m.get(k) {
    Some(v) => d_arr(v)
    None => []
  }
}

// ---- encode primitives -----------------------------------------------------

fn j_esc(s : String) -> String {
  let mut out = "\""
  for c in s {
    if c == '"' {
      out = out + "\\\""
    } else if c == '\\' {
      out = out + "\\\\"
    } else if c == '\n' {
      out = out + "\\n"
    } else if c == '\r' {
      out = out + "\\r"
    } else if c == '\t' {
      out = out + "\\t"
    } else {
      out = out + c.to_string()
    }
  }
  out + "\""
}

fn j_arr(parts : Array[String]) -> String {
  let mut out = "["
  for i in 0..<parts.length() {
    if i > 0 {
      out = out + ","
    }
    out = out + parts[i]
  }
  out + "]"
}
