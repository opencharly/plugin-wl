package wl

import (
	"context"
	"os"
	"path/filepath"
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
		"hypr-monitors", "hypr-clients", "hypr-layers", "hypr-workspaces",
		"hypr-systeminfo", "hypr-dispatch", "hypr-eval",
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

// A compositor that exports session state to its CHILDREN rather than carrying it
// in its own environ cannot be served by the /proc/<pid>/environ lift alone, because
// that environ is frozen at exec. Hyprland is the live case: without recovery every
// hyprctl call fails with "HYPRLAND_INSTANCE_SIGNATURE not set! (is hyprland
// running?)" even though Hyprland is running and detection has already resolved it.
// Observed on a live nested Hyprland 0.56.2 bed before this recovery existed.
func TestEnvPreludeRecoversHyprlandInstanceSignature(t *testing.T) {
	hypr := profileByName(t, "hyprland")
	if hypr.EnvRecover == "" {
		t.Fatal("the hyprland profile declares no EnvRecover: HYPRLAND_INSTANCE_SIGNATURE " +
			"is absent from Hyprland's own /proc/<pid>/environ, so every hypr-* method fails")
	}

	prelude := envPrelude()
	// The recovery must be present, guarded on the detected process, and must run
	// AFTER XDG_RUNTIME_DIR is defaulted -- it reads $XDG_RUNTIME_DIR/hypr.
	if !strings.Contains(prelude, "HYPRLAND_INSTANCE_SIGNATURE=") {
		t.Error("env prelude never assigns HYPRLAND_INSTANCE_SIGNATURE")
	}
	if !strings.Contains(prelude, `$XDG_RUNTIME_DIR/hypr`) {
		t.Error("env prelude does not read the instance directory, the only source of the signature")
	}
	guard := `if [ "$__c" = 'Hyprland' ]; then`
	if !strings.Contains(prelude, guard) {
		t.Errorf("hyprland recovery is not guarded on the detected process; want %q", guard)
	}
	if strings.Index(prelude, `XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp}"`) >
		strings.Index(prelude, `$XDG_RUNTIME_DIR/hypr`) {
		t.Error("recovery runs before XDG_RUNTIME_DIR is defaulted, so it reads /hypr off an empty path")
	}

	// A profile with no recovery must emit no guard at all -- the table stays data
	// driven, and an unrelated compositor pays nothing for Hyprland's quirk.
	kwin := profileByName(t, "kwin")
	if kwin.EnvRecover != "" {
		t.Error("kwin declares an EnvRecover it does not need")
	}
	if strings.Contains(prelude, `if [ "$__c" = 'kwin_wayland' ]; then`) {
		t.Error("env prelude emits an empty recovery guard for kwin")
	}
}

// hypr-keyword must NOT come back. On a Lua-config Hyprland (>= 0.55, i.e. every
// version this plugin targets) `hyprctl keyword` refuses with "keyword can't work
// with non-legacy parsers. Use eval." AND EXITS 0 - so the method reported success
// while changing nothing, which is worse than failing. hypr-eval replaces it.
// Measured live on 0.56.2.
func TestHyprKeywordIsNotOffered(t *testing.T) {
	caps, err := NewMeta().Describe(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	if strings.Contains(caps.GetSchemaCue(), `"hypr-keyword"`) {
		t.Error("hypr-keyword is back in the #WlInput enum; on a Lua-config Hyprland it " +
			"exits 0 without applying anything")
	}
	if _, ok := requiredModifiers["hypr-keyword"]; ok {
		t.Error("hypr-keyword still declares requiredModifiers")
	}
}

// R5 sweep: `hyprctl keyword` must not survive ANYWHERE. It is not merely
// deprecated on a Lua-config Hyprland - it answers "keyword can't work with
// non-legacy parsers. Use eval." and EXITS 0, so every call site reports success
// while applying nothing. Removing the hypr-keyword method was not enough: the
// resolution SET path shipped the same command. Measured on 0.56.2:
//
//	$ hyprctl keyword monitor "WAYLAND-1,1600x900,auto,1"
//	keyword can't work with non-legacy parsers. Use eval.   # rc=0
func TestNoHyprctlKeywordSurvives(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		// Match the command STRING (a Go literal opens with a quote), so the
		// comment explaining why the command is banned does not trip the guard.
		if strings.Contains(string(b), `"hyprctl keyword`) {
			t.Errorf("%s still issues `hyprctl keyword`: it exits 0 without applying "+
				"anything on a Lua-config Hyprland. Use `hyprctl eval` with an hl.* call", f)
		}
	}
}

func profileByName(t *testing.T, name string) compositorProfile {
	t.Helper()
	for _, p := range compositorProfiles {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no %q profile in the compositor table", name)
	return compositorProfile{}
}

// TestHyprlandProfile pins the capability set Hyprland actually has. Unlike KWin,
// it implements every wlroots protocol the tooling needs, so nothing must
// fail-fast.
// TestWakeOutputCmd proves the DPMS-wake command construction: Hyprland uses
// the Lua dispatcher (hl.dsp.dpms({action = "enable"}) — the legacy argv is
// rejected under the Lua config manager), wlroots uses wlr-randr. Fails without
// the wakeOutput fix (a DPMS-off output makes grim hang forever).
func TestWakeOutputCmd(t *testing.T) {
	hypr := wakeOutputCmd(resolutionHyprctl, "Virtual-1")
	if !strings.Contains(hypr, "hl.dsp.dpms") || !strings.Contains(hypr, "enable") {
		t.Fatalf("Hyprland wake command: want the Lua dpms dispatcher, got: %s", hypr)
	}
	wlr := wakeOutputCmd(resolutionWlrRandr, "DP-1")
	if !strings.Contains(wlr, "wlr-randr --output") || !strings.Contains(wlr, "--on") {
		t.Fatalf("wlroots wake command: want wlr-randr --on, got: %s", wlr)
	}
	if empty := wakeOutputCmd(resolutionWlrRandr, ""); empty != "" {
		t.Fatalf("wlroots wake with no output: want empty, got: %s", empty)
	}
}

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
