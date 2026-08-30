package wl

import (
	"strings"
	"testing"
)

// TestHyprlandWindowActionsAreAimedByFocus pins the one property that makes the
// Hyprland window verbs correct, and that nothing else in the package can check.
//
// Every hl.dsp.window.* dispatcher operates on the ACTIVE window. Some of them
// ACCEPT a selector argument and then ignore it, which is the dangerous shape:
// the call returns ok, so the action reports success against a window it never
// touched. Measured on 0.56.2 with four windows mapped:
//
//	focused C, then hl.dsp.window.close("initialtitle:D")   -> C died,  D survived
//	focused A, then hl.dsp.window.move({selector = "B", …})  -> A moved, B stayed
//
// So the selector has to be applied by FOCUS, and the action must carry none.
// Both halves are asserted, because each fails differently: dropping the focus
// aims the action at whatever was focused, and putting the selector back on the
// action makes it silently inert again.
//
// This test FAILS against the pre-fix code, whose close emitted
// `hl.dsp.window.close(<selector>)` with no focus at all.
func TestHyprlandWindowActionsAreAimedByFocus(t *testing.T) {
	const target = "initialtitle:cstream-focus-a"

	for _, tc := range []struct {
		verb   string
		action string
	}{
		{"close", "hl.dsp.window.close()"},
		{"fullscreen", "hl.dsp.window.fullscreen(1)"},
		{"minimize", `hl.dsp.window.move({workspace = "special:minimized"})`},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			got := hyprWindowActionExprs(target, tc.action)

			// 1. focus FIRST, and by the `window` key. `{selector=…}`, `{address=…}`
			// and `{target=…}` all return nil from hl.dsp.focus -- silently, so the
			// wrong key surfaces one layer away as "expected a dispatcher".
			if !strings.HasPrefix(got[0], `hl.dsp.focus({window = "`) {
				t.Fatalf("%s: first expression must focus the target by the `window` key, got %q",
					tc.verb, got[0])
			}
			if !strings.Contains(got[0], target) {
				t.Fatalf("%s: focus expression does not carry the target: %q", tc.verb, got[0])
			}

			// 2. the ACTION must carry no selector. This is the half that fails
			// against the old code, and the failure it prevents is invisible at
			// runtime: the action succeeds, on the wrong window.
			if strings.Contains(got[1], target) {
				t.Fatalf("%s: the action carries a selector (%q). Hyprland ACCEPTS it and "+
					"IGNORES it, so the action would hit the focused window while reporting "+
					"success against %q", tc.verb, got[1], target)
			}
			if got[1] != tc.action {
				t.Fatalf("%s: action expression rewritten: want %q, got %q", tc.verb, tc.action, got[1])
			}
		})
	}
}

// TestHyprMinimizeWorkspaceIsNamed pins the minimize destination.
//
// Hyprland has no minimize state; a special workspace is the compositor's idiom
// for it. The name matters twice over: the anonymous `special` would collide with
// a user's own special workspace, and a NAMED one is what makes the effect
// observable -- it appears in hl.get_workspaces() once the first window lands
// there, which is what a check can assert instead of absence of error.
func TestHyprMinimizeWorkspaceIsNamed(t *testing.T) {
	if !strings.HasPrefix(hyprMinimizeWorkspace, "special:") {
		t.Fatalf("minimize must target a special workspace, got %q", hyprMinimizeWorkspace)
	}
	if hyprMinimizeWorkspace == "special:" {
		t.Fatal("minimize must target a NAMED special workspace, not the anonymous one")
	}
}
