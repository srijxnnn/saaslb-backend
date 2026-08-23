package domain

import (
	"strings"
	"testing"
)

func TestNormalizeVisitorID(t *testing.T) {
	t.Parallel()

	ok := "2c1a9f3e-4b8d-4f21-9c0a-7e5d6b1a2c3d"
	got, err := NormalizeVisitorID("  " + ok + "  ")
	if err != nil {
		t.Fatalf("valid id: %v", err)
	}
	if got != ok {
		t.Fatalf("got %q", got)
	}

	if _, err := NormalizeVisitorID("xxxxxxxx"); err != nil {
		t.Fatalf("8-char id should pass: %v", err)
	}

	for _, in := range []string{"", "short", "bad id!", strings.Repeat("x", 65)} {
		if _, err := NormalizeVisitorID(in); err != ErrBadVisitor {
			t.Fatalf("%q: error = %v, want %v", in, err, ErrBadVisitor)
		}
	}
}
