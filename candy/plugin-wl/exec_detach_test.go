package wl

import (
	"strings"
	"testing"
)

// `wl: exec` exists to start GUI apps — things that map a window and keep running.
// A trailing `&` alone does not do that: the backgrounded child inherits stdout and
// stderr, so the pipe wlCapture reads never reaches EOF and the capture blocks for the
// full deadline. Measured on a live Hyprland guest against the pre-fix code:
//
//	FAIL  the terminal launches and maps a window
//	      wl: exec: launching "foot": rpc error: code = DeadlineExceeded
//
// while the NEXT step found foot's window mapped. The app was fine; the verb could not
// report it. That is the worst shape a check verb can have — a false failure for correct
// behaviour — so both halves are pinned here: the redirect (which closes the pipe) and
// setsid (which detaches the child so it outlives the launching shell).
func TestExecCommandDetachesAndRedirects(t *testing.T) {
	got := wlExecCommand("foot")

	// stdout AND stderr must be redirected — inheriting EITHER holds the pipe open.
	if !strings.Contains(got, ">/dev/null 2>&1") {
		t.Errorf("exec line does not redirect stdout+stderr, so the capture pipe never closes:\n%s", got)
	}
	// stdin closed: an app that reads stdin would otherwise block on the capture's.
	if !strings.Contains(got, "</dev/null") {
		t.Errorf("exec line does not close stdin:\n%s", got)
	}
	// Detached, so the app survives the shell that launched it.
	if !strings.Contains(got, "setsid ") {
		t.Errorf("exec line does not detach the child with setsid:\n%s", got)
	}
	// Still backgrounded.
	if !strings.HasSuffix(strings.TrimSpace(got), "&") {
		t.Errorf("exec line is not backgrounded:\n%s", got)
	}
	// XWayland apps still need a DISPLAY.
	if !strings.Contains(got, "DISPLAY=:0") {
		t.Errorf("exec line dropped DISPLAY, breaking XWayland apps:\n%s", got)
	}
}

// The command may carry arguments and must NOT be shell-quoted as one word, or every
// multi-argument launch (`chromium --new-window`) would try to exec a binary whose name
// contains a space.
func TestExecCommandKeepsArgumentsUnquoted(t *testing.T) {
	got := wlExecCommand("chromium --new-window")
	if !strings.Contains(got, "chromium --new-window") {
		t.Errorf("exec line mangled a command with arguments:\n%s", got)
	}
}
