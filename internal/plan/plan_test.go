package plan_test

// RED: plan YAML -> snake_case plan JSON must reproduce the fixture plan.json
// (semantically). Fails until YAMLToJSON is implemented.

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/afewell-hh/aid/internal/fixtures"
	"github.com/afewell-hh/aid/internal/plan"
)

func TestYAMLToJSON_MatchesFixtureJSON(t *testing.T) {
	for _, name := range []string{"clos-small", "mesh-two-switch", "switch-bom"} {
		yamlBytes, err := fixtures.PlanYAML("valid", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		gotJSON, err := plan.YAMLToJSON(yamlBytes)
		if err != nil {
			t.Fatalf("%s: YAMLToJSON: %v", name, err)
		}
		wantJSON, err := fixtures.PlanJSON("valid", name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got, want any
		if err := json.Unmarshal(gotJSON, &got); err != nil {
			t.Fatalf("%s: parse got: %v", name, err)
		}
		if err := json.Unmarshal(wantJSON, &want); err != nil {
			t.Fatalf("%s: parse want: %v", name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: YAML->JSON does not match fixture plan.json", name)
		}
	}
}
