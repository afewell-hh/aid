package orchestrate

import "encoding/json"

// Wire types for the D16 JSON envelopes. Field names follow the snake_case wire
// contract (IR_CONTRACT.md / BOM_CONTRACT.md; the kernel encoder emits these).

// CalcResult is the kernel output envelope: {"ok":<calc-output>} | {"err":...}.
type CalcResult struct {
	Ok  *CalcOutput     `json:"ok,omitempty"`
	Err json.RawMessage `json:"err,omitempty"`
}

// CalcOutput mirrors wit `calc-output` = { ir, boms, validation }.
type CalcOutput struct {
	IR         TopologyIR        `json:"ir"`
	Boms       []json.RawMessage `json:"boms"`
	Validation ValidationResult  `json:"validation"`
}

// TopologyIR is the calculator IR (only the fields the CLI needs to inspect;
// the full object is forwarded verbatim to the hhfab adapter).
type TopologyIR struct {
	Metadata json.RawMessage   `json:"metadata"`
	Nodes    []json.RawMessage `json:"nodes"`
	Edges    []json.RawMessage `json:"edges"`
	Fabrics  []json.RawMessage `json:"fabrics"`
}

// ValidationResult mirrors wit `validation-result`.
type ValidationResult struct {
	IsValid  bool              `json:"is_valid"`
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// ValidationIssue mirrors wit `validation-issue`.
type ValidationIssue struct {
	Severity string  `json:"severity"`
	Code     string  `json:"code"`
	Message  string  `json:"message"`
	EntryID  *string `json:"entry_id,omitempty"`
	FabricID *string `json:"fabric_id,omitempty"`
	ZoneID   *string `json:"zone_id,omitempty"`
}

// hhfabResult is the hhfab adapter output envelope.
type hhfabResult struct {
	Ok  *HhfabOutput    `json:"ok,omitempty"`
	Err json.RawMessage `json:"err,omitempty"`
}

// HhfabOutput = { documents: [{ fabric, yaml }] }.
type HhfabOutput struct {
	Documents []WiringDocument `json:"documents"`
}

// WiringDocument is one fabric's wiring YAML.
type WiringDocument struct {
	Fabric string `json:"fabric"`
	YAML   string `json:"yaml"`
}

// bomResult is the bom adapter output envelope.
type bomResult struct {
	Ok  *BomOutput      `json:"ok,omitempty"`
	Err json.RawMessage `json:"err,omitempty"`
}

// BomOutput = { format, content }.
type BomOutput struct {
	Format  string `json:"format"`
	Content string `json:"content"`
}
