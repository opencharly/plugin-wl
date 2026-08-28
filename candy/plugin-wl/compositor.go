package wl

import (
	"context"
	"fmt"
	"strings"

	"github.com/opencharly/sdk"
	"github.com/opencharly/spec/shellquote"
)

// compositor.go — the per-compositor capability table.
//
// Supporting a compositor used to mean editing every method: `detectCompositor`
// was a two-valued string ("kwin" | "wlroots") consulted by seventeen inline
// `== "kwin"` guards, and the session-environment prelude carried a hard-coded
// process list. A fourth compositor multiplied both.
//
// Compositor support is now DATA. One `compositorProfile` row declares which
// backend serves each capability and which capabilities have no backend at all;
// the method bodies switch on the capability, never on a compositor name. Adding
// a compositor is one entry in `compositorProfiles` plus, if it needs a tool no
// existing row uses, one case in the corresponding dispatch.

// windowBackend is the tool that serves window management (list/focus/close/
// fullscreen/minimize/geometry).
type windowBackend string

const (
	windowWlrctl  windowBackend = "wlrctl"  // wlr-foreign-toplevel-management
	windowKdotool windowBackend = "kdotool" // KWin scripting over D-Bus
	windowHyprctl windowBackend = "hyprctl" // Hyprland IPC
)

// resolutionBackend is the tool that serves output-mode changes. Empty means the
// compositor implements no protocol we can drive.
type resolutionBackend string

const (
	resolutionWlrRandr resolutionBackend = "wlr-randr" // wlr-output-management
	resolutionHyprctl  resolutionBackend = "hyprctl"   // Hyprland IPC
	resolutionNone     resolutionBackend = ""
)

// compositorProfile is the whole contract for one compositor.
//
// A false capability is not a gap in this plugin — it means the compositor
// implements no protocol we can drive host-safely, and the matching method
// fail-fasts with `unsupportedReason` instead of hanging. That distinction
// matters: the wlroots tools do not error on KWin, they BLOCK.
type compositorProfile struct {
	// Name is what `wl: status` reports.
	Name string
	// Process is the pgrep -x target that identifies a running instance, and the
	// process whose /proc/<pid>/environ supplies the session environment.
	Process string
	// ExtraEnv are additional environment variables to lift from that process on
	// top of the common set (XDG_RUNTIME_DIR, WAYLAND_DISPLAY,
	// DBUS_SESSION_BUS_ADDRESS).
	ExtraEnv []string
	// EnvRecover is a shell snippet that reconstructs session environment the
	// compositor does NOT carry in its own /proc/<pid>/environ, run after the
	// common set has been lifted and defaulted.
	//
	// Hyprland is what forces this to exist: it exports
	// HYPRLAND_INSTANCE_SIGNATURE to the processes it SPAWNS, but
	// /proc/<pid>/environ is frozen at exec time, so the signature is simply not
	// there to lift and every hyprctl call fails with "HYPRLAND_INSTANCE_SIGNATURE
	// not set! (is hyprland running?)". The instance directory under
	// $XDG_RUNTIME_DIR/hypr/ is the actual source of truth. Keeping the snippet in
	// the table row rather than branching on the compositor name is what lets a
	// future compositor with the same problem cost one field, not one more
	// if-statement.
	EnvRecover string

	Window     windowBackend
	Resolution resolutionBackend
	Pointer    bool
	Keyboard   bool
	Clipboard  bool

	// unsupportedReason explains, per capability, why there is no backend. Keyed
	// by the capability names used in unsupportedErr.
	unsupportedReason map[string]string
}

// compositorProfiles is consulted in order; the first whose Process is running
// wins. Order therefore matters only where two compositors could run at once.
var compositorProfiles = []compositorProfile{
	{
		Name:       "kwin",
		Process:    "kwin_wayland",
		Window:     windowKdotool,
		Resolution: resolutionNone,
		Pointer:    false,
		Keyboard:   false,
		Clipboard:  false,
		unsupportedReason: map[string]string{
			"pointer": "no host-safe backend; KWin 6 removed org_kde_kwin_fake_input, the " +
				"RemoteDesktop portal is approval-gated, and /dev/uinput leaks into the host",
			"keyboard":   "wtype needs zwp_virtual_keyboard_manager_v1, which KWin does not implement",
			"clipboard":  "wl-clipboard needs wlr-data-control, which KWin does not implement",
			"resolution": "wlr-randr needs wlr-output-management, which KWin does not implement; kscreen-doctor has no working backend here and hangs",
		},
	},
	{
		// Hyprland implements every wlroots protocol the tooling needs
		// (zwp_virtual_keyboard_manager_v1, zwlr_virtual_pointer_manager_v1,
		// zwlr_foreign_toplevel_manager_v1, zwlr_data_control_manager_v1 AND
		// ext_data_control_manager_v1, zwlr_output_manager_v1,
		// zwlr_screencopy_manager_v1), so nothing fail-fasts. Window management and
		// resolution prefer hyprctl: it is richer and avoids the stale-IPC-socket
		// class of bug that affects sway's socket discovery.
		Name:     "hyprland",
		Process:  "Hyprland",
		ExtraEnv: []string{"HYPRLAND_INSTANCE_SIGNATURE"},
		// Hyprland never has the signature in its own environ (see EnvRecover), so
		// recover it from the instance directory. Newest wins, the same rule
		// sway's socket discovery uses: a restarted compositor leaves the previous
		// instance's directory behind, and picking the older one silently targets a
		// dead session.
		EnvRecover: `if [ -z "${HYPRLAND_INSTANCE_SIGNATURE:-}" ] && [ -d "$XDG_RUNTIME_DIR/hypr" ]; then ` +
			`__sig=$(ls -t "$XDG_RUNTIME_DIR/hypr" 2>/dev/null | head -1); ` +
			`[ -n "$__sig" ] && export HYPRLAND_INSTANCE_SIGNATURE="$__sig"; fi; `,
		Window:     windowHyprctl,
		Resolution: resolutionHyprctl,
		Pointer:    true,
		Keyboard:   true,
		Clipboard:  true,
	},
	{
		Name:       "sway",
		Process:    "sway",
		Window:     windowWlrctl,
		Resolution: resolutionWlrRandr,
		Pointer:    true,
		Keyboard:   true,
		Clipboard:  true,
	},
	{
		Name:       "labwc",
		Process:    "labwc",
		Window:     windowWlrctl,
		Resolution: resolutionWlrRandr,
		Pointer:    true,
		Keyboard:   true,
		Clipboard:  true,
	},
}

// wlrootsProfile is the fallback for an unrecognised wlroots-family compositor:
// assume the wlroots protocol set, which is what the tooling targets anyway.
var wlrootsProfile = compositorProfile{
	Name:       "wlroots",
	Window:     windowWlrctl,
	Resolution: resolutionWlrRandr,
	Pointer:    true,
	Keyboard:   true,
	Clipboard:  true,
}

// unsupportedErr reports that a capability has no backend on this compositor,
// with the reason from the profile. Callers get a clear error instead of the
// indefinite hang the wlroots tools produce on a compositor lacking the protocol.
func (p compositorProfile) unsupportedErr(capability, method string) error {
	reason := p.unsupportedReason[capability]
	if reason == "" {
		reason = fmt.Sprintf("%s is not supported on %s", capability, p.Name)
	}
	return fmt.Errorf("wl %s: %s on %s (%s). Run `wl: status` to see which capabilities this compositor supports",
		method, capability, p.Name, reason)
}

// envPrelude builds the shell prelude that sources the RUNNING compositor's real
// session environment before applying safe fallbacks. Load-bearing for KWin's
// random dbus-run-session bus and for Hyprland's instance signature, and a strict
// improvement everywhere else.
//
// Generated from compositorProfiles so a new compositor's process name and extra
// environment are picked up automatically.
func envPrelude() string {
	seen := map[string]bool{
		"XDG_RUNTIME_DIR":          true,
		"WAYLAND_DISPLAY":          true,
		"DBUS_SESSION_BUS_ADDRESS": true,
	}
	vars := []string{"XDG_RUNTIME_DIR", "WAYLAND_DISPLAY", "DBUS_SESSION_BUS_ADDRESS"}
	procs := make([]string, 0, len(compositorProfiles))
	for _, p := range compositorProfiles {
		procs = append(procs, p.Process)
		for _, v := range p.ExtraEnv {
			if !seen[v] {
				seen[v] = true
				vars = append(vars, v)
			}
		}
	}
	prelude := fmt.Sprintf(
		`for __c in %s; do __p=$(pgrep -x "$__c" 2>/dev/null | head -1); [ -n "$__p" ] && break; done; `,
		strings.Join(procs, " "),
	) +
		fmt.Sprintf(
			`if [ -n "$__p" ] && [ -r /proc/$__p/environ ]; then eval "$(tr '\0' '\n' < /proc/$__p/environ | grep -E '^(%s)=' | sed 's/^/export /')"; fi; `,
			strings.Join(vars, "|"),
		) +
		`export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp}" WAYLAND_DISPLAY="${WAYLAND_DISPLAY:-wayland-0}"; `

	// Per-profile recovery for session state that is NOT in the compositor's
	// environ. Runs last, so it can rely on XDG_RUNTIME_DIR being set, and is
	// guarded on the profile whose process was actually found.
	for _, p := range compositorProfiles {
		if p.EnvRecover == "" {
			continue
		}
		prelude += fmt.Sprintf(`if [ "$__c" = %s ]; then %s fi; `,
			shellquote.ShellQuote(p.Process), p.EnvRecover)
	}
	return strings.TrimSuffix(prelude, "; ")
}

// detectCompositor returns the profile of the compositor running on the venue,
// falling back to the generic wlroots profile. The probe runs raw (no env
// prelude — it needs no Wayland environment itself).
func detectCompositor(ctx context.Context, ex *sdk.Executor) compositorProfile {
	if !ex.VenueHasTool(ctx, "pgrep") {
		// Detection is impossible without pgrep, and silently claiming "wlroots"
		// would be dangerous: on a KWin venue the wlroots tools do not error, they
		// BLOCK. Return the conservative fallback but NAME the reason so `wl: status`
		// reports it instead of asserting a compositor it never observed.
		p := wlrootsProfile
		p.Name = "unknown (pgrep unavailable — install procps/procps-ng)"
		return p
	}
	for _, p := range compositorProfiles {
		if ex.VenueRunSilent(ctx, fmt.Sprintf("pgrep -x %s >/dev/null 2>&1", shellquote.ShellQuote(p.Process))) == nil {
			return p
		}
	}
	return wlrootsProfile
}

// yesNo renders a capability flag for `wl: status`.
func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// resolutionLabel renders the resolution backend for `wl: status`, naming the
// absent case rather than printing an empty string.
func resolutionLabel(r resolutionBackend) string {
	if r == resolutionNone {
		return "unsupported"
	}
	return string(r)
}

// defaultOutputName is the last-resort output name: what gst-wayland-display
// calls its own output. Used only when discovery finds nothing.
const defaultOutputName = "HEADLESS-1"

// primaryOutputName discovers the compositor's first output, asking the backend
// the profile declares. Output names are NOT universal - gst-wayland-display uses
// HEADLESS-1, a nested Hyprland uses WAYLAND-1, a real DRM head uses DP-1 - so
// anything that names an output must discover it.
func primaryOutputName(ctx context.Context, ex *sdk.Executor) string {
	switch detectCompositor(ctx, ex).Resolution {
	case resolutionHyprctl:
		if data, err := wlCapture(ctx, ex,
			`hyprctl -j monitors | tr -d " " | grep -m1 '"name"' | cut -d'"' -f4`); err == nil {
			if name := strings.TrimSpace(data); name != "" {
				return name
			}
		}
	case resolutionWlrRandr:
		if data, err := wlCapture(ctx, ex, "wlr-randr 2>/dev/null | head -1"); err == nil {
			if fields := strings.Fields(strings.TrimSpace(data)); len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return defaultOutputName
}
