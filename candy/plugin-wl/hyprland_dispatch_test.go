package wl

import (
	"strings"
	"testing"
)

// TestHyprWindowActionExprCarriesSelectorInTable pins the one property that
// makes the Hyprland window verbs aim at the RIGHT window, which nothing else in
// the package can check.
//
// `hl.dsp.window.*` takes its target in a TABLE. The POSITIONAL form
// (`hl.dsp.window.close("initialtitle:D")`) ACCEPTS the selector and ignores it —
// measured on 0.56.2 with two windows mapped: with the OTHER window focused, the
// positional form closed the wrong window. An earlier reading generalized from
// that form and applied the selector by FOCUS instead. That is both unnecessary
// and unreliable: the TABLE form (`hl.dsp.window.close({window="class:D"})`)
// honors the selector directly, and was measured closing only the named window
// while the focused one survived. Focus-then-act also cannot work on a headless
// bed, where `activewindow` is null.
//
// The selector is normalized: a bare `target: foot` (what every bed authors, and
// what the wlrctl backend matches as the app id) becomes `class:foot`, because a
// bare string matches NOTHING in a Hyprland action table.
//
// This test FAILS against the pre-fix code, whose expression aimed the action by
// a preceding focus and therefore carried no selector on the action itself.
func TestHyprWindowActionExprCarriesSelectorInTable(t *testing.T) {
	for _, action := range []string{"close", "fullscreen", "move"} {
		t.Run(action, func(t *testing.T) {
			got := hyprWindowActionExpr("omawrite", action)

			// 1. the action TABLE carries `window = "class:<target>"`.
			wantPrefix := "hl.dsp.window." + action + `({window = "class:omawrite"`
			if !strings.HasPrefix(got, wantPrefix) {
				t.Fatalf("%s: expression must aim the action by the table's `window` key with the bare target normalized to class:, got %q", action, got)
			}
			// 2. it must NOT be the positional form, which silently ignores the selector.
			if strings.Contains(got, `(`+`"`) {
				t.Fatalf("%s: expression used the positional form, whose selector Hyprland ignores: %q", action, got)
			}
		})
	}
}

// TestHyprSelectorNormalization pins the bare-target → class: normalization and
// the pass-through of an explicit selector. Without the first, an authored
// `target: foot` silently does nothing on Hyprland while working on sway/labwc;
// without the second, a bed's explicit `initialtitle:`/`address:` is corrupted.
func TestHyprSelectorNormalization(t *testing.T) {
	if got := hyprSelector("foot"); got != "class:foot" {
		t.Fatalf("bare target must normalize to class:, got %q", got)
	}
	for _, explicit := range []string{
		"class:foot", "initialclass:foot", "title:foo", "initialtitle:foo",
		"address:0xabc", "regex:foo", "pid:123",
	} {
		if got := hyprSelector(explicit); got != explicit {
			t.Fatalf("explicit selector %q must pass through unchanged, got %q", explicit, got)
		}
	}
}

// TestHyprDispatchOutcome pins the stdout contract that makes the window verbs
// trustworthy on Hyprland.
//
// `hyprctl dispatch` exits 0 whether the dispatch worked or not — it writes
// `ok` on success and `error: …` / `warning: …` on failure to STDOUT, and an
// unresolvable selector (the exact bug this PR fixes) produces
// `warning: hl.focus: window not found` with exit 0. A caller that trusted the
// exit status reported "Closed window matching X" for a window that was still
// open. This classifier is the whole fix: exactly `ok` is success, anything
// else — including an empty body — is a failure. Measured live on 0.56.2.
func TestHyprDispatchOutcome(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		wantErr bool
	}{
		{"clean ok", "ok\n", false},
		{"ok without trailing newline", "ok", false},
		{"ok with surrounding whitespace", "  ok  \n", false},
		{"focus window-not-found (the RCA case)", "warning: =[C]:-1: hl.focus: window not found\n", true},
		{"error line", "error: =[C]:-1: hl.window.fullscreen: invalid mode \"2\" (expected fullscreen/maximized)\n", true},
		{"dispatcher error", "error: return hl.dispatch(...):1: hl.dispatch: expected a dispatcher (e.g. hl.dsp.window.close())\n", true},
		{"empty body", "", true},
		{"whitespace-only body", "  \n\t", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := hyprDispatchOutcome(tc.out)
			if tc.wantErr && err == nil {
				t.Fatalf("hyprDispatchOutcome(%q) = nil, want an error", tc.out)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("hyprDispatchOutcome(%q) = %v, want nil", tc.out, err)
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
