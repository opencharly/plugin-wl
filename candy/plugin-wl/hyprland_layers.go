package wl

// hyprland_layers.go implements the `hypr-layers` method: layer-shell surfaces, with an
// ON-SCREEN verdict computed per surface.
//
// WHY THIS EXISTS AT ALL. `hypr-clients` (and `toplevel`, and `windows`) can only ever see
// TOPLEVEL windows. On a Quickshell/wlroots desktop the bar, the notification popups, the
// launcher, the OSD, the wallpaper and the lockscreen are none of them toplevels — they are
// zwlr_layer_shell_v1 surfaces, and they are invisible to every method this plugin had. A
// desktop bed could assert that a terminal opened but could not assert that the BAR exists.
//
// WHY IT COMPUTES `onscreen` RATHER THAN RETURNING RAW JSON. The other hypr-* methods return
// `hyprctl -j <query>` verbatim, and for layers that is not enough to write a check against.
// A layer surface can be MAPPED but parked outside the monitor: that is exactly how a hidden
// bar is implemented, because unmapping and remapping would rebuild the surface. So
// "the bar is hidden" and "the bar is showing" are the SAME JSON except for coordinates, and
// deciding between them means comparing the surface box against the monitor's logical bounds
// — scale-divided, and axis-swapped when the monitor is rotated. That is a correlation across
// two different hyprctl queries; no `stdout:` matcher can do it, and pushing it into every
// bed as a jq incantation would put the same geometry in N places and require jq in the guest.
//
// So the method emits one TAB-SEPARATED LINE per surface, ending in `onscreen` or `offscreen`:
//
//	omarchy-bar         HEADLESS-1  0  0     1920  40    onscreen
//	omarchy-background  HEADLESS-1  0  0     1920  1080  onscreen
//	omarchy-bar         HEADLESS-1  0  -40   1920  40    offscreen
//
// which makes every assertion a bed actually needs a single matcher:
//
//	present    stdout: {contains: "omarchy-bar"}
//	absent     stdout: {not_contains: "omarchy-bar"}
//	showing    stdout: {matches: "omarchy-bar\\s.*\\sonscreen"}
//	hidden     stdout: {matches: "omarchy-bar\\s.*\\soffscreen"}
//
// The plaintext-record shape follows the `mcp` verb's precedent ("tab-separated plaintext so
// matchers can contains: without JSON decoding"), for the same reason.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/opencharly/sdk"
)

// hyprMonitor is the subset of `hyprctl -j monitors` this needs. Hyprland reports width and
// height in PHYSICAL pixels; layer surface boxes are in the monitor's LOGICAL coordinate
// space, so the bounds must be scale-divided before they can be compared.
type hyprMonitor struct {
	Name      string  `json:"name"`
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Scale     float64 `json:"scale"`
	Transform int     `json:"transform"`
}

// hyprLayerSurface is one layer-shell surface as `hyprctl -j layers` reports it.
type hyprLayerSurface struct {
	Namespace string  `json:"namespace"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	W         float64 `json:"w"`
	H         float64 `json:"h"`
}

// hyprLayerLevels is the per-monitor body: {"levels": {"0": [...], "2": [...]}}.
type hyprLayerLevels struct {
	Levels map[string][]hyprLayerSurface `json:"levels"`
}

// logicalBounds returns the monitor's logical width and height — the coordinate space layer
// surface boxes live in.
//
// An ODD `transform` is a 90° or 270° rotation, which swaps the axes: a 1920x1080 panel
// rotated to portrait presents a 1080x1920 logical area, and a bar pinned to its top is
// 1080 wide, not 1920. Ignoring transform would call a correctly-placed bar off-screen on
// every rotated monitor.
//
// A zero or absent scale means "not reported"; treat it as 1 rather than dividing by zero,
// which would make every bound +Inf and every surface trivially on-screen.
func (m hyprMonitor) logicalBounds() (w, h float64) {
	scale := m.Scale
	if scale <= 0 {
		scale = 1
	}
	pw, ph := m.Width, m.Height
	if m.Transform%2 != 0 {
		pw, ph = ph, pw
	}
	return math.Round(pw / scale), math.Round(ph / scale)
}

// onScreen reports whether a surface box INTERSECTS the monitor's logical area at all.
//
// Intersection, not containment: a bar that is 1 pixel taller than its reserved strip, or a
// notification sliding in from the edge, is genuinely visible and must not be called hidden.
// The failure this distinguishes is the deliberate park — a hidden bar sits at y = -h, fully
// outside — and any intersection test separates those two cleanly.
func onScreen(s hyprLayerSurface, boundsW, boundsH float64) bool {
	return s.X+s.W > 0 && s.X < boundsW &&
		s.Y+s.H > 0 && s.Y < boundsH
}

// renderHyprLayers turns the two hyprctl payloads into the tab-separated record set.
// Split from the venue call so tests drive the real formatter on real captured JSON rather
// than a reimplementation of it.
func renderHyprLayers(layersJSON, monitorsJSON string) (string, error) {
	var monitors []hyprMonitor
	if err := json.Unmarshal([]byte(monitorsJSON), &monitors); err != nil {
		return "", fmt.Errorf("hypr-layers: decoding monitors: %w", err)
	}
	bounds := make(map[string][2]float64, len(monitors))
	for _, m := range monitors {
		w, h := m.logicalBounds()
		bounds[m.Name] = [2]float64{w, h}
	}

	var byMonitor map[string]hyprLayerLevels
	if err := json.Unmarshal([]byte(layersJSON), &byMonitor); err != nil {
		return "", fmt.Errorf("hypr-layers: decoding layers: %w", err)
	}

	// Deterministic order: monitor, then level, then the surface order Hyprland reported.
	// A check that greps one namespace does not care, but a check that asserts the whole
	// set — or a human diffing two runs — does, and map iteration would reshuffle it.
	monNames := make([]string, 0, len(byMonitor))
	for name := range byMonitor {
		monNames = append(monNames, name)
	}
	sort.Strings(monNames)

	var b strings.Builder
	for _, mon := range monNames {
		bw, bh := 0.0, 0.0
		if bd, ok := bounds[mon]; ok {
			bw, bh = bd[0], bd[1]
		}
		levels := byMonitor[mon].Levels
		lvNames := make([]string, 0, len(levels))
		for lv := range levels {
			lvNames = append(lvNames, lv)
		}
		sort.Strings(lvNames)

		for _, lv := range lvNames {
			for _, s := range levels[lv] {
				// A monitor named in `layers` but absent from `monitors` has no bounds to
				// compare against. Reporting `unknown` rather than guessing keeps a bed
				// from reading a fabricated verdict as a real one — an `onscreen` matcher
				// does not match it, so the check fails loudly instead of passing blind.
				verdict := "unknown"
				if bw > 0 && bh > 0 {
					verdict = "offscreen"
					if onScreen(s, bw, bh) {
						verdict = "onscreen"
					}
				}
				fmt.Fprintf(&b, "%s\t%s\t%g\t%g\t%g\t%g\t%s\n",
					s.Namespace, mon, s.X, s.Y, s.W, s.H, verdict)
			}
		}
	}
	return b.String(), nil
}

// wlHyprLayers is the method body: two hyprctl queries, then the shared formatter.
func wlHyprLayers(ctx context.Context, ex *sdk.Executor) (string, error) {
	layersJSON, err := hyprctlJSON(ctx, ex, "layers")
	if err != nil {
		return "", err
	}
	monitorsJSON, err := hyprctlJSON(ctx, ex, "monitors")
	if err != nil {
		return "", err
	}
	return renderHyprLayers(layersJSON, monitorsJSON)
}
