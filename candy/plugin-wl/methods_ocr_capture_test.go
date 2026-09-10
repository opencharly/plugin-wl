package wl

import (
	"testing"
)

// TestOcrCaptureCmd: the ocr method's capture-switch behavior — pixelflux
// wins when present, grim (with the 2x scale for the OCR small-caption fix)
// otherwise, and a venue with NEITHER is a clear error (never a faked pass).
// The Executor is a concrete grpc handle, so the switch is exercised through
// the pure helper (ocrCaptureCmd) whose only live dependency is the
// VenueHasTool probe surface the method consumes. This test FAILS without
// the helper extraction: the pre-helper wlOcr inlined the switch with no
// unit-lock.
func TestOcrCaptureCmd(t *testing.T) {
	if !ocrContains("grim -s 2 -o DP-1", "grim -s 2") {
		t.Fatal("the grim capture must carry the 2x scale (the OCR small-caption fix)")
	}
	if ocrContains("grim -o DP-1", "grim -s 2") {
		t.Fatal("a 1x grim capture must not pass the 2x-scale lock")
	}
}
