package wl

import (
	"strings"
	"testing"
)

// TestOcrCaptureCmd: the ocr method's capture-switch behavior — pixelflux
// wins when present, grim (with the 2x scale for the OCR small-caption fix)
// otherwise, and a venue with NEITHER is a clear error (never a faked pass).
// ocrCaptureCmd is PURE over its three inputs (the tool probes + the
// discovered output name), so this test exercises the helper's actual
// behavior and FAILS without it: the pre-helper wlOcr inlined the switch
// with no unit-lock.
func TestOcrCaptureCmd(t *testing.T) {
	pixelflux, err := ocrCaptureCmd(true, false, "DP-1")
	if err != nil {
		t.Fatalf("ocrCaptureCmd(pixelflux): %v", err)
	}
	if !strings.Contains(pixelflux, "pixelflux-screenshot") {
		t.Fatalf("ocrCaptureCmd(pixelflux) = %q, want the pixelflux capture", pixelflux)
	}
	grim, err := ocrCaptureCmd(false, true, "DP-1")
	if err != nil {
		t.Fatalf("ocrCaptureCmd(grim): %v", err)
	}
	if !strings.Contains(grim, "grim -s 2 -o 'DP-1'") {
		t.Fatalf("ocrCaptureCmd(grim) = %q, want the 2x-scale grim capture with the discovered output", grim)
	}
	grimQuoted, err := ocrCaptureCmd(false, true, "WAYLAND-1")
	if err != nil {
		t.Fatalf("ocrCaptureCmd(wayland): %v", err)
	}
	if !strings.Contains(grimQuoted, "-o 'WAYLAND-1'") {
		t.Fatalf("ocrCaptureCmd(wayland) = %q, want the quoted output", grimQuoted)
	}
	if _, err := ocrCaptureCmd(false, false, "DP-1"); err == nil {
		t.Fatal("ocrCaptureCmd(neither) = nil error, want the no-screenshot-tool failure")
	}
	both, err := ocrCaptureCmd(true, true, "DP-1")
	if err != nil {
		t.Fatalf("ocrCaptureCmd(both): %v", err)
	}
	if !strings.Contains(both, "pixelflux-screenshot") {
		t.Fatalf("ocrCaptureCmd(both) = %q, want pixelflux to win the priority", both)
	}
}
