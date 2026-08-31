package wl

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/opencharly/plugin-wl/candy/plugin-wl/params"
)

// The sdk's artifact pipeline reads this modifier out of the desugared plugin-input MAP by
// string key: inputString(op, "artifact_contains_text"). Nothing type-checks that lookup
// against this plugin's schema — they meet only at runtime, through a map.
//
// So a rename or typo on either side does not fail to compile and does not error at runtime.
// The sdk simply finds no key, reads "", and SKIPS the validator: the step passes while
// asserting nothing. That is the silent-skip shape this repo has been bitten by before (a
// candy var in a check step, an unrequired verb), and it is exactly what an OCR assertion
// must not do — its whole purpose is catching a surface that mapped without rendering.
//
// This pins the wire name the schema generates against the literal the sdk looks up.
func TestArtifactContainsText_WireNameMatchesTheSdkLookupKey(t *testing.T) {
	const sdkLookupKey = "artifact_contains_text"

	b, err := json.Marshal(params.WlInput{ArtifactContainsText: "SAN FRANCISCO"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round map[string]any
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got, ok := round[sdkLookupKey]
	if !ok {
		t.Fatalf("WlInput does not serialise a %q key, so the sdk validator would read \"\" and\n"+
			"SKIP the assertion silently. Serialised keys: %v", sdkLookupKey, keysOf(round))
	}
	if got != "SAN FRANCISCO" {
		t.Errorf("%s round-tripped to %v, want the authored text", sdkLookupKey, got)
	}
}

// The schema is the single source the params struct is generated from, so the field has to
// exist there too — a params struct edited by hand would pass the test above while the
// authored YAML failed validation against the served schema.
func TestArtifactContainsText_DeclaredInTheServedSchema(t *testing.T) {
	b, err := schemaFS.ReadFile("schema/wl.cue")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !strings.Contains(string(b), "artifact_contains_text?:") {
		t.Error("schema/wl.cue declares no artifact_contains_text field, so an authored step using " +
			"it would be REJECTED by the host's plugin-input validation")
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
