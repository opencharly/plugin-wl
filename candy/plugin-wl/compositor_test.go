package wl

import (
	"context"
	"strings"
	"testing"

	pb "github.com/opencharly/spec/proto"
)

// TestSchemaCompilesAndCoversEveryMethod is the load-bearing guard for the method
// enum: the SDK compiles the served CUE in BuildCapabilities, so a malformed
// schema fails here rather than at plugin-load time on a live deployment. It also
// asserts the enum and the Go dispatch cannot drift apart — an authored method
// missing from the enum fails `charly box validate`, and an enum entry with no
// dispatch case is a runtime "unknown wl method".
func TestSchemaCompilesAndCoversEveryMethod(t *testing.T) {
	caps, err := NewMeta().Describe(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("Describe failed - the served CUE schema does not compile: %v", err)
	}
	schema := caps.GetSchemaCue()
	if schema == "" {
		t.Fatal("no CUE schema served")
	}
	for _, m := range []string{
		"hypr-monitors", "hypr-clients", "hypr-workspaces", "hypr-systeminfo",
		"hypr-dispatch", "hypr-keyword", "hypr-eval",
	} {
		if !strings.Contains(schema, `"`+m+`"`) {
			t.Errorf("method %q is dispatched but missing from the #WlInput enum; "+
				"authoring it would fail `charly box validate`", m)
		}
	}
	// Every method that declares required modifiers must be in the enum too.
	for m := range requiredModifiers {
		if !strings.Contains(schema, `"`+m+`"`) {
			t.Errorf("method %q has requiredModifiers but is not in the #WlInput enum", m)
		}
	}
}

// TestCompositorProfilesAreWellFormed keeps the capability table honest: an
// unsupported capability must carry a reason, because that reason is the whole
// error a user sees when a method is refused.
func TestCompositorProfilesAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range compositorProfiles {
		if p.Name == "" || p.Process == "" {
			t.Errorf("profile %+v must declare both Name and Process", p)
		}
		if seen[p.Name] {
			t.Errorf("duplicate compositor profile %q", p.Name)
		}
		seen[p.Name] = true
		if p.Window == "" {
			t.Errorf("%s: Window backend must be set", p.Name)
		}
		for capability, supported := range map[string]bool{
			"pointer": p.Pointer, "keyboard": p.Keyboard, "clipboard": p.Clipboard,
		} {
			if !supported && p.unsupportedReason[capability] == "" {
				t.Errorf("%s: %s is unsupported but carries no reason; the reason IS the "+
					"user-visible error", p.Name, capability)
			}
		}
		if p.Resolution == resolutionNone && p.unsupportedReason["resolution"] == "" {
			t.Errorf("%s: resolution is unsupported but carries no reason", p.Name)
		}
	}
}

// TestEnvPreludeCoversEveryProfile is the regression guard for the bug this table
// replaced: the prelude used to carry a hard-coded process list, so a compositor
// added to the table but missing from that list ran every wl: command against the
// WRONG display, silently.
func TestEnvPreludeCoversEveryProfile(t *testing.T) {
	prelude := envPrelude()
	for _, p := range compositorProfiles {
		if !strings.Contains(prelude, p.Process) {
			t.Errorf("env prelude does not source %s (%s): every wl: command would "+
				"fall back to WAYLAND_DISPLAY=wayland-0 and target the wrong display",
				p.Name, p.Process)
		}
		for _, v := range p.ExtraEnv {
			if !strings.Contains(prelude, v) {
				t.Errorf("env prelude does not lift %s, required by %s", v, p.Name)
			}
		}
	}
	for _, v := range []string{"XDG_RUNTIME_DIR", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS"} {
		if !strings.Contains(prelude, v) {
			t.Errorf("env prelude lost the common variable %s", v)
		}
	}
}

// TestHyprlandProfile pins the capability set Hyprland actually has. Unlike KWin,
// it implements every wlroots protocol the tooling needs, so nothing must
// fail-fast.
func TestHyprlandProfile(t *testing.T) {
	var hypr *compositorProfile
	for i := range compositorProfiles {
		if compositorProfiles[i].Name == "hyprland" {
			hypr = &compositorProfiles[i]
		}
	}
	if hypr == nil {
		t.Fatal("no hyprland profile")
	}
	if !hypr.Pointer || !hypr.Keyboard || !hypr.Clipboard {
		t.Error("Hyprland implements zwlr_virtual_pointer_manager_v1, " +
			"zwp_virtual_keyboard_manager_v1 and wlr/ext data-control - none of these " +
			"may fail-fast")
	}
	if hypr.Resolution != resolutionHyprctl || hypr.Window != windowHyprctl {
		t.Errorf("Hyprland should route window+resolution through hyprctl, got %s/%s",
			hypr.Window, hypr.Resolution)
	}
	if hypr.Process != "Hyprland" {
		t.Errorf("the process name is capital-H Hyprland, got %q", hypr.Process)
	}
}

// TestKWinStillFailsFast guards the behaviour the table replaced: the wlroots
// tools do not error on KWin, they BLOCK, so these must be refused explicitly.
func TestKWinStillFailsFast(t *testing.T) {
	var kwin *compositorProfile
	for i := range compositorProfiles {
		if compositorProfiles[i].Name == "kwin" {
			kwin = &compositorProfiles[i]
		}
	}
	if kwin == nil {
		t.Fatal("no kwin profile")
	}
	if kwin.Pointer || kwin.Keyboard || kwin.Clipboard || kwin.Resolution != resolutionNone {
		t.Error("KWin implements none of wlr-virtual-pointer, virtual-keyboard, " +
			"wlr-data-control or wlr-output-management; marking any supported reintroduces " +
			"the indefinite hang")
	}
	if kwin.Window != windowKdotool {
		t.Errorf("KWin window management goes through kdotool, got %s", kwin.Window)
	}
	err := kwin.unsupportedErr("pointer", "click")
	if !strings.Contains(err.Error(), "kwin") || !strings.Contains(err.Error(), "click") {
		t.Errorf("unsupported error should name compositor and method, got %q", err)
	}
}

var _ = context.Background
