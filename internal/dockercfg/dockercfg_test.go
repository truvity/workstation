package dockercfg

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The property this port exists to preserve. A rewrite that marshals a
// fresh config passes any test that only checks credHelpers — and
// silently destroys the developer's auths, credsStore and plugin
// settings. They find out at the next docker login.
func TestApplyPreservesUnrelatedKeys(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{
		"auths":       map[string]any{"ghcr.io": map[string]any{"auth": "secret"}},
		"credsStore":  "pass",
		"psFormat":    "table {{.ID}}",
		"credHelpers": map[string]any{"existing.example.com": "other-helper"},
	}

	Apply(cfg, []string{"1.dkr.ecr.eu-central-1.amazonaws.com"})

	for _, key := range []string{"auths", "credsStore", "psFormat"} {
		if _, ok := cfg[key]; !ok {
			t.Fatalf("Apply dropped %q — unrelated config must survive", key)
		}
	}

	helpers, _ := Helpers(cfg)
	if got := helpers["existing.example.com"]; got != "other-helper" {
		t.Fatalf("Apply clobbered an unrelated credHelper: got %v", got)
	}

	if got := helpers["1.dkr.ecr.eu-central-1.amazonaws.com"]; got != "ecr-login" {
		t.Fatalf("Apply did not set the ECR helper: got %v", got)
	}
}

// A hand-edited credHelpers that is not an object must be reported, not
// silently replaced — replacing without a word loses someone's work.
func TestHelpersReportsNonObject(t *testing.T) {
	t.Parallel()

	if _, ok := Helpers(map[string]any{"credHelpers": "not-an-object"}); ok {
		t.Fatal("Helpers accepted a non-object credHelpers as valid")
	}

	if _, ok := Helpers(map[string]any{}); !ok {
		t.Fatal("Helpers reported absent credHelpers as invalid")
	}
}

// Missing is what --check reports on, so it has to notice a host mapped
// to the WRONG helper, not merely an absent one.
func TestMissingDetectsWrongHelper(t *testing.T) {
	t.Parallel()

	cfg := map[string]any{"credHelpers": map[string]any{
		"right.example.com": "ecr-login",
		"wrong.example.com": "some-other-helper",
	}}

	got := Missing(cfg, []string{"right.example.com", "wrong.example.com", "absent.example.com"})

	want := []string{"wrong.example.com", "absent.example.com"}
	if len(got) != len(want) {
		t.Fatalf("Missing = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Missing = %v, want %v", got, want)
		}
	}
}

func TestWriteAtomicRoundTrips(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "sub", "config.json")

	cfg := map[string]any{"credHelpers": map[string]any{"a.example.com": "ecr-login"}}
	if err := WriteAtomic(path, cfg); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if perm := info.Mode().Perm(); perm != filePerm {
		t.Fatalf("mode = %v, want %v — this file holds registry credentials", perm, os.FileMode(filePerm))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}

	if _, ok := back["credHelpers"]; !ok {
		t.Fatal("round trip lost credHelpers")
	}

	// No temp files left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected only config.json, found %d entries", len(entries))
	}
}
