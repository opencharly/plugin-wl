package wl

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"

	pb "github.com/opencharly/spec/proto"
)

// TestEveryDispatchedMethodIsInTheEnum closes the drift hole the hand-written list in
// TestSchemaCompilesAndCoversEveryMethod leaves open.
//
// That test enumerates the methods it checks BY HAND, so it can only ever verify the
// methods somebody remembered to add to it. Adding a `case "x":` to the dispatch switch and
// forgetting the CUE enum leaves both tests green — and the failure surfaces on a live
// deployment as `charly box validate` rejecting an authored step, which is the worst
// place to find it and the furthest from the change that caused it.
//
// This derives the set from the dispatch switch itself by scanning methods.go, so the
// guard cannot fall behind the code it guards. It is the same shape as charly's own
// TestPacstrapBuilderTemplateFieldsExist, which regexes charly.yml for template field
// references and requires each to be a real struct field.
func TestEveryDispatchedMethodIsInTheEnum(t *testing.T) {
	src, err := os.ReadFile("methods.go")
	if err != nil {
		t.Fatalf("reading methods.go: %v", err)
	}
	body := string(src)

	// Bound the scan to the dispatch switch. `case "get":`/`"set":`/`"up":`/`"middle":`
	// further down belong to the clipboard-action, scroll-direction and pointer-button
	// sub-switches — they are argument VALUES, not verb methods, and requiring them in
	// the method enum would be wrong.
	start := strings.Index(body, "func dispatch(")
	if start < 0 {
		t.Fatal("the dispatch switch (func dispatch) was not found in methods.go — this guard needs updating")
	}
	end := strings.Index(body[start:], "\n}\n")
	if end < 0 {
		t.Fatal("could not find the end of the dispatch switch")
	}
	dispatch := body[start : start+end]

	methods := regexp.MustCompile(`case "([a-z][a-z0-9-]*)":`).FindAllStringSubmatch(dispatch, -1)
	if len(methods) < 30 {
		t.Fatalf("only %d dispatch cases found — the scan is not seeing the switch, "+
			"so this guard would pass vacuously", len(methods))
	}

	caps, err := NewMeta().Describe(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("Describe failed — the served CUE schema does not compile: %v", err)
	}
	schema := caps.GetSchemaCue()

	for _, m := range methods {
		if !strings.Contains(schema, `"`+m[1]+`"`) {
			t.Errorf("method %q has a dispatch case but is absent from the #WlInput enum: "+
				"authoring it would be rejected by `charly box validate`", m[1])
		}
	}
}

// The mirror direction: an enum entry with no dispatch case is a method a user can
// author, pass validation with, and then hit "unknown wl method" against a live
// deployment — a failure that costs a whole bed cycle to discover.
func TestEveryEnumMethodIsDispatched(t *testing.T) {
	src, err := os.ReadFile("methods.go")
	if err != nil {
		t.Fatalf("reading methods.go: %v", err)
	}
	body := string(src)

	caps, err := NewMeta().Describe(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}
	schema := caps.GetSchemaCue()

	// The method enum is the `method:` line of #WlInput.
	line := regexp.MustCompile(`(?m)^\s*method:\s*(.+)$`).FindStringSubmatch(schema)
	if line == nil {
		t.Fatal("could not find the method: enum line in the served schema")
	}
	names := regexp.MustCompile(`"([a-z][a-z0-9-]*)"`).FindAllStringSubmatch(line[1], -1)
	if len(names) < 30 {
		t.Fatalf("only %d enum entries parsed — the scan is not seeing the enum", len(names))
	}

	for _, n := range names {
		if !strings.Contains(body, `case "`+n[1]+`":`) {
			t.Errorf("method %q is in the #WlInput enum but has no dispatch case: "+
				"authoring it passes validation and then fails on a live deployment", n[1])
		}
	}
}
