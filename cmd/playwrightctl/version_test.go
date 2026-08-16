package main

import (
	"sort"
	"testing"
)

// String ordering puts "1.9.0" above "1.10.0", which would recommend an OLDER
// release as the newest shared version — the exact wrong answer for the
// decision this tool exists to make. Compare numerically per component.
func TestLessIsNumericNotLexical(t *testing.T) {
	t.Parallel()

	cases := []struct {
		a, b string
		want bool
	}{
		{"1.9.0", "1.10.0", true},   // the lexical trap
		{"1.58.2", "1.61.1", true},  // the real pair this tool was written for
		{"1.61.1", "1.62.1", true},
		{"1.10.0", "1.9.0", false},
		{"1.61.1", "1.61.1", false}, // equal is not less
		{"2.0.0", "1.99.99", false},
	}

	for _, tc := range cases {
		if got := less(tc.a, tc.b); got != tc.want {
			t.Errorf("less(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// The whole point is picking the newest version BOTH sides can supply. With
// nixpkgs at 1.61.1 and npm at 1.62.1 the answer is 1.61.1 — picking npm's
// newest is the mistake that breaks at run time, not install time.
func TestNewestSharedVersion(t *testing.T) {
	t.Parallel()

	nix := []string{"1.61.1", "1.60.0", "1.59.1", "1.58.2"}
	npm := []string{"1.62.1", "1.61.1", "1.60.0", "1.59.1", "1.58.2", "1.9.0"}

	have := make(map[string]bool, len(npm))
	for _, v := range npm {
		have[v] = true
	}

	var common []string

	for _, v := range nix {
		if have[v] {
			common = append(common, v)
		}
	}

	sort.Slice(common, func(i, j int) bool { return less(common[j], common[i]) })

	if len(common) == 0 || common[0] != "1.61.1" {
		t.Fatalf("newest shared = %v, want 1.61.1 first", common)
	}
}
