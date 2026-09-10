package wl

import (
	"strings"
	"testing"
)

// TestOcrCaptureCmd: the ocr method's capture-switch behavior — pixelflux
// wins when present, grim otherwise (with the authored scale — 0 = the native
// resolution, 2 = the upstream screen_contains small-caption fix), and a
// venue with NEITHER is a clear error (never a faked pass). ocrCaptureCmd is
// PURE over its four inputs, so this test exercises the helper's actual
// behavior and FAILS without it.
func TestOcrCaptureCmd(t *testing.T) {
	pixelflux, err := ocrCaptureCmd(true, false, "DP-1", 0)
	if err != nil {
		t.Fatalf("ocrCaptureCmd(pixelflux): %v", err)
	}
	if !strings.Contains(pixelflux, "pixelflux-screenshot") {
		t.Fatalf("ocrCaptureCmd(pixelflux) = %q, want the pixelflux capture", pixelflux)
	}
	native, err := ocrCaptureCmd(false, true, "DP-1", 0)
	if err != nil {
		t.Fatalf("ocrCaptureCmd(native): %v", err)
	}
	if !strings.Contains(native, "grim -o 'DP-1'") || strings.Contains(native, "-s 2") {
		t.Fatalf("ocrCaptureCmd(native) = %q, want the unscaled capture", native)
	}
	scaled, err := ocrCaptureCmd(false, true, "DP-1", 2)
	if err != nil {
		t.Fatalf("ocrCaptureCmd(scaled): %v", err)
	}
	if !strings.Contains(scaled, "grim -s 2 -o 'DP-1'") {
		t.Fatalf("ocrCaptureCmd(scaled) = %q, want the 2x-scale capture", scaled)
	}
	if _, err := ocrCaptureCmd(false, false, "DP-1", 0); err == nil {
		t.Fatal("ocrCaptureCmd(neither) = nil error, want the no-screenshot-tool failure")
	}
	both, err := ocrCaptureCmd(true, true, "DP-1", 0)
	if err != nil {
		t.Fatalf("ocrCaptureCmd(both): %v", err)
	}
	if !strings.Contains(both, "pixelflux-screenshot") {
		t.Fatalf("ocrCaptureCmd(both) = %q, want pixelflux to win the priority", both)
	}
}
