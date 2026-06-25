package main

import (
	"github.com/afewell-hh/aid/internal/design"
	"github.com/afewell-hh/aid/internal/planedit"
)

// planFacts is the at-a-glance derived summary (P2.1, #71) shown on the plan list
// rows and the detail header — all engine-derived, never guessed. Computable is
// false when the plan fails structural ingest, so the calc-derived facts
// (switch_total, is_valid) are unknown and the surface shows them as "—".
type planFacts struct {
	Topology    string `json:"topology"` // mesh | Clos
	GpuCount    int    `json:"gpu_count"`
	ServerTotal int    `json:"server_total"`
	SwitchTotal int    `json:"switch_total"`
	IsValid     bool   `json:"is_valid"`
	Computable  bool   `json:"computable"`
}

// computeFacts derives the summary for a plan. GPU count + server total are
// plan-static (planedit projection: quantity × gpus_per_server); switch total +
// validity come from a calc (design.Validate); topology is Clos iff the plan has
// a spine tier (hedgehog_role spine, or an explicit clos/spine-leaf topology_mode
// — xoc Clos plans carry no topology_mode, they declare spine roles).
func computeFacts(trainingYAML []byte) planFacts {
	f := planFacts{Topology: "mesh"}
	if proj, err := planedit.Project(trainingYAML); err == nil {
		for _, sc := range proj.ServerClasses {
			f.ServerTotal += sc.Quantity
			f.GpuCount += sc.Quantity * sc.GpusPerServer
		}
	}
	res, err := design.Validate(design.Inputs{TrainingYAML: trainingYAML})
	if err != nil {
		return f // structural failure — calc facts unknown (Computable stays false)
	}
	f.Computable = true
	f.IsValid = res.Valid()
	for _, q := range res.Calc.SwitchQuantity {
		f.SwitchTotal += q.Quantity
	}
	for _, sw := range res.Plan.Spec.SwitchClasses {
		if sw.HedgehogRole == "spine" || sw.TopologyMode == "clos" || sw.TopologyMode == "spine-leaf" {
			f.Topology = "Clos"
		}
	}
	return f
}
