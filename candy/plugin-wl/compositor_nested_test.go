package wl

import "testing"

// A nested compositor's own environ carries the PARENT's WAYLAND_DISPLAY -- the
// socket it connects TO, not the one it SERVES. Inheriting that points every wl:
// call at the parent, and the result is a silent wrong-target rather than an
// error. Measured against a gst-wayland-display parent: grim on the inherited
// display returned "compositor doesn't support the screen capture protocol",
// while the same grim on Hyprland's own socket captured a 2.5 MB frame.

func TestPreludeResolvesHyprlandsOwnSocket(t *testing.T) {
	p := envPrelude()

	// It must consult hyprctl for the socket Hyprland actually serves...
	if !containsAll(p, `hyprctl -j instances`, `wl_socket`) {
		t.Errorf("prelude does not resolve Hyprland's served socket via hyprctl:\n%s", p)
	}
	// ...and export it, or the inherited parent value stays in force.
	if !containsAll(p, `export WAYLAND_DISPLAY="$__own"`) {
		t.Errorf("prelude resolves the served socket but never exports it:\n%s", p)
	}
}

// The override must be CONDITIONAL. On a non-nested compositor hyprctl may be
// absent or report nothing, and clobbering WAYLAND_DISPLAY with an empty value
// would break the very case that works today.
func TestOwnSocketOverrideIsGuarded(t *testing.T) {
	p := envPrelude()
	if !containsAll(p, `[ -n "$__own" ] && export WAYLAND_DISPLAY`) {
		t.Errorf("the served-socket override is unguarded; an empty result would clobber a working display:\n%s", p)
	}
}

// The generic fallbacks must still be in place for compositors with no such
// self-report.
func TestPreludeKeepsGenericFallbacks(t *testing.T) {
	p := envPrelude()
	if !containsAll(p, `WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}"`) {
		t.Errorf("the generic WAYLAND_DISPLAY fallback is gone:\n%s", p)
	}
	if !containsAll(p, `XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp}"`) {
		t.Errorf("the generic XDG_RUNTIME_DIR fallback is gone:\n%s", p)
	}
}

// Ordering is the contract: the served-socket override runs in EnvRecover, which
// the prelude appends AFTER the environ import and after the generic fallbacks.
// If it ran earlier the environ import would overwrite it again.
func TestOwnSocketOverrideRunsAfterTheEnvironImport(t *testing.T) {
	p := envPrelude()
	imp := indexOf(p, `/proc/$__p/environ`)
	fallback := indexOf(p, `WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}"`)
	own := indexOf(p, `export WAYLAND_DISPLAY="$__own"`)
	if imp < 0 || fallback < 0 || own < 0 {
		t.Fatalf("prelude is missing one of the three stages (import=%d fallback=%d own=%d)", imp, fallback, own)
	}
	if !(own > fallback && fallback > imp) {
		t.Errorf("the served-socket override must run last (import=%d fallback=%d own=%d); "+
			"running earlier lets the inherited parent value win again", imp, fallback, own)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if indexOf(s, sub) < 0 {
			return false
		}
	}
	return true
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
