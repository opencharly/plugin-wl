package wl

import (
	"strings"
	"testing"
)

// The monitors payload every case below shares: one 1920x1080 unscaled, untransformed head,
// which is what a headless nested Hyprland presents.
const monitorsPlain = `[{"name":"HEADLESS-1","width":1920,"height":1080,"scale":1,"transform":0}]`

func layersFor(body string) string {
	return `{"HEADLESS-1":{"levels":{"2":[` + body + `]}}}`
}

// The whole reason this method exists: a hidden bar is MAPPED and present in `hyprctl -j
// layers` exactly like a visible one — it is parked outside the monitor instead of being
// unmapped, so the surface does not have to be rebuilt when it is revealed. If the verdict
// were "is the namespace present", both states would read identically and the check would
// pass either way.
func TestRenderHyprLayers_ParkedBarIsOffScreenNotAbsent(t *testing.T) {
	shown := layersFor(`{"namespace":"omarchy-bar","x":0,"y":0,"w":1920,"h":40}`)
	hidden := layersFor(`{"namespace":"omarchy-bar","x":0,"y":-40,"w":1920,"h":40}`)

	outShown, err := renderHyprLayers(shown, monitorsPlain)
	if err != nil {
		t.Fatalf("render shown: %v", err)
	}
	outHidden, err := renderHyprLayers(hidden, monitorsPlain)
	if err != nil {
		t.Fatalf("render hidden: %v", err)
	}

	// Present in BOTH — this is the trap.
	for name, out := range map[string]string{"shown": outShown, "hidden": outHidden} {
		if !strings.Contains(out, "omarchy-bar") {
			t.Fatalf("%s: the bar surface is missing entirely: %q", name, out)
		}
	}
	if !strings.Contains(outShown, "onscreen") {
		t.Errorf("a bar at y=0 must be onscreen, got %q", outShown)
	}
	if !strings.Contains(outHidden, "offscreen") {
		t.Errorf("a bar parked at y=-h must be offscreen, got %q", outHidden)
	}
	if strings.Contains(outHidden, "\tonscreen") {
		t.Errorf("the parked bar was reported onscreen: %q", outHidden)
	}
}

// A 90°/270° rotation swaps the logical axes. Without honouring transform, a bar correctly
// pinned across the top of a portrait monitor (1080 logical wide) is measured against a
// 1920-wide bound and still passes — but a surface parked just outside the REAL 1080 edge
// would also pass, which is the assertion silently going blind.
func TestRenderHyprLayers_RotatedMonitorSwapsBounds(t *testing.T) {
	rotated := `[{"name":"HEADLESS-1","width":1920,"height":1080,"scale":1,"transform":1}]`
	// x=1100 is beyond the portrait logical width (1080) but well inside 1920.
	beyond := layersFor(`{"namespace":"probe","x":1100,"y":0,"w":100,"h":40}`)

	out, err := renderHyprLayers(beyond, rotated)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "offscreen") {
		t.Errorf("x=1100 is outside a rotated monitor's 1080 logical width; "+
			"transform was not honoured. got %q", out)
	}
}

// Layer boxes are in LOGICAL coordinates; hyprctl reports monitor size in PHYSICAL pixels.
// On a 2x display a 1920-logical-wide monitor reports width 3840, so an unscaled comparison
// would call everything on-screen — including a surface genuinely parked off the edge.
func TestRenderHyprLayers_ScaleDividesBounds(t *testing.T) {
	hidpi := `[{"name":"HEADLESS-1","width":3840,"height":2160,"scale":2,"transform":0}]`
	// x=2000 is outside the 1920 LOGICAL width, inside the 3840 physical one.
	beyond := layersFor(`{"namespace":"probe","x":2000,"y":0,"w":100,"h":40}`)

	out, err := renderHyprLayers(beyond, hidpi)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "offscreen") {
		t.Errorf("x=2000 exceeds the 1920 logical width of a 2x 3840px monitor; "+
			"scale was not divided out. got %q", out)
	}
}

// A surface that merely OVERLAPS the edge is visible and must not be called hidden — the
// state this method distinguishes is the deliberate full park, not "touches an edge".
func TestRenderHyprLayers_PartiallyVisibleCountsAsOnScreen(t *testing.T) {
	straddling := layersFor(`{"namespace":"toast","x":-50,"y":0,"w":200,"h":80}`)
	out, err := renderHyprLayers(straddling, monitorsPlain)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "onscreen") {
		t.Errorf("a surface straddling the left edge is visible; got %q", out)
	}
}

// NEGATIVE CONTROL on the fallback. A monitor present in `layers` but absent from `monitors`
// has no bounds, and guessing either verdict would hand a bed a fabricated answer. `unknown`
// matches neither an onscreen nor an offscreen matcher, so the check fails loudly.
func TestRenderHyprLayers_UnknownMonitorIsNotGuessed(t *testing.T) {
	orphan := `{"DP-9":{"levels":{"2":[{"namespace":"ghost","x":0,"y":0,"w":10,"h":10}]}}}`
	out, err := renderHyprLayers(orphan, monitorsPlain)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "unknown") {
		t.Fatalf("a surface on a monitor with no reported geometry must be 'unknown', got %q", out)
	}
	if strings.Contains(out, "onscreen") || strings.Contains(out, "offscreen") {
		t.Errorf("an unknown-geometry surface was given a real verdict: %q", out)
	}
}

// Output order must be stable: Go map iteration is randomised, so without an explicit sort
// two runs against an unchanged desktop would emit different byte streams.
func TestRenderHyprLayers_OutputIsDeterministic(t *testing.T) {
	monitors := `[{"name":"A","width":800,"height":600,"scale":1,"transform":0},
	              {"name":"B","width":800,"height":600,"scale":1,"transform":0}]`
	layers := `{"B":{"levels":{"2":[{"namespace":"b2","x":0,"y":0,"w":10,"h":10}],
	                            "0":[{"namespace":"b0","x":0,"y":0,"w":10,"h":10}]}},
	            "A":{"levels":{"1":[{"namespace":"a1","x":0,"y":0,"w":10,"h":10}]}}}`
	first, err := renderHyprLayers(layers, monitors)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for i := 0; i < 20; i++ {
		again, err := renderHyprLayers(layers, monitors)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if again != first {
			t.Fatalf("output is not deterministic:\n%q\nvs\n%q", first, again)
		}
	}
	// A before B, and within A/B the levels ascend.
	if idxA, idxB := strings.Index(first, "a1"), strings.Index(first, "b0"); idxA > idxB {
		t.Errorf("monitors are not name-sorted: %q", first)
	}
	if idx0, idx2 := strings.Index(first, "b0"), strings.Index(first, "b2"); idx0 > idx2 {
		t.Errorf("levels are not sorted within a monitor: %q", first)
	}
}

// The record shape IS the API — every bed assertion is a matcher over these fields, so a
// change to the column order or the separator silently breaks every consuming check.
func TestRenderHyprLayers_RecordShapeIsTabSeparated(t *testing.T) {
	out, err := renderHyprLayers(
		layersFor(`{"namespace":"omarchy-bar","x":0,"y":0,"w":1920,"h":40}`), monitorsPlain)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	line := strings.TrimSuffix(out, "\n")
	got := strings.Split(line, "\t")
	want := []string{"omarchy-bar", "HEADLESS-1", "0", "0", "1920", "40", "onscreen"}
	if len(got) != len(want) {
		t.Fatalf("want %d tab-separated fields, got %d: %q", len(want), len(got), line)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("field %d: got %q, want %q (line %q)", i, got[i], want[i], line)
		}
	}
}

// Malformed input must be an ERROR, never an empty pass. An empty string satisfies
// `not_contains:` matchers, so a silently-empty result would turn every absence assertion
// in every consuming bed green at once.
func TestRenderHyprLayers_MalformedInputErrors(t *testing.T) {
	if _, err := renderHyprLayers("not json", monitorsPlain); err == nil {
		t.Error("malformed layers JSON must error, not return empty output")
	}
	if _, err := renderHyprLayers(layersFor(""), "not json"); err == nil {
		t.Error("malformed monitors JSON must error, not return empty output")
	}
}

// A zero or missing scale must not divide by zero — that would make every bound +Inf and
// every surface trivially on-screen, which is the assertion failing open.
func TestRenderHyprLayers_ZeroScaleIsTreatedAsOne(t *testing.T) {
	noScale := `[{"name":"HEADLESS-1","width":1920,"height":1080,"transform":0}]`
	out, err := renderHyprLayers(
		layersFor(`{"namespace":"probe","x":5000,"y":0,"w":10,"h":10}`), noScale)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(out, "offscreen") {
		t.Errorf("with scale absent the bounds must stay 1920x1080, so x=5000 is offscreen; got %q", out)
	}
}
