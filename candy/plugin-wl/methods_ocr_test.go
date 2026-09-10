package wl

import "testing"

// TestOcrContains: the ocr method's text assertion — upstream
// screen_contains's grep -F -i semantics (case-insensitive contains).
func TestOcrContains(t *testing.T) {
	if !ocrContains("Weather 12°\nNotifications", "notifications") {
		t.Fatal("ocrContains(case-insensitive) = false, want true")
	}
	if !ocrContains("exact", "exact") {
		t.Fatal("ocrContains(exact) = false, want true")
	}
	if ocrContains("no match here", "missing") {
		t.Fatal("ocrContains(absent) = true, want false")
	}
}

// TestWlOcrRequiredText: the ocr method requires the text modifier (the
// dispatch-level required-modifier check).
func TestWlOcrRequiredModifiers(t *testing.T) {
	if req := requiredModifiers["ocr"]; len(req) != 1 || req[0] != "text" {
		t.Fatalf("requiredModifiers[ocr] = %v, want [text]", req)
	}
}
