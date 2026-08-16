package direnvcfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathPrefixCovers(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		dir   string
		want  bool
	}{
		{
			name:  "exact match",
			entry: "/a/b",
			dir:   "/a/b",
			want:  true,
		},
		{
			name:  "covers child",
			entry: "/a/b",
			dir:   "/a/b/c",
			want:  true,
		},
		{
			name:  "covers deep descendant",
			entry: "/home/x/github/truvity",
			dir:   "/home/x/github/truvity/bar/worktree",
			want:  true,
		},
		{
			name:  "no cover on sibling with shared string prefix",
			entry: "/a/b",
			dir:   "/a/bc",
			want:  false,
		},
		{
			name:  "no cover when entry is deeper than dir",
			entry: "/a/b/c",
			dir:   "/a/b",
			want:  false,
		},
		{
			name:  "no cover for unrelated path",
			entry: "/a/b",
			dir:   "/x/y",
			want:  false,
		},
		{
			name:  "trailing slash on entry is cleaned",
			entry: "/a/b/",
			dir:   "/a/b/c",
			want:  true,
		},
		{
			name:  "trailing slash on dir is cleaned",
			entry: "/a/b",
			dir:   "/a/b/",
			want:  true,
		},
		{
			name:  "unclean entry is cleaned before comparison",
			entry: "/a/./b/../b",
			dir:   "/a/b/c",
			want:  true,
		},
		{
			name:  "root covers everything",
			entry: "/",
			dir:   "/a/b",
			want:  true,
		},
		{
			name:  "boundary check with multi-segment suffix",
			entry: "/home/x/github/truvity",
			dir:   "/home/x/github/truvity-fork/bar",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathPrefixCovers(tt.entry, tt.dir); got != tt.want {
				t.Errorf("pathPrefixCovers(%q, %q) = %v, want %v", tt.entry, tt.dir, got, tt.want)
			}
		})
	}
}

func TestWhitelistCovers(t *testing.T) {
	tests := []struct {
		name      string
		wl        direnvWhitelist
		dir       string
		wantEntry string
		wantOK    bool
	}{
		{
			name:   "empty whitelist",
			wl:     direnvWhitelist{},
			dir:    "/a/b",
			wantOK: false,
		},
		{
			name:      "parent prefix covers",
			wl:        direnvWhitelist{Prefix: []string{"/home/x/github/truvity"}},
			dir:       "/home/x/github/truvity/bar/worktree",
			wantEntry: "/home/x/github/truvity",
			wantOK:    true,
		},
		{
			name:      "exact same prefix covers",
			wl:        direnvWhitelist{Prefix: []string{"/a/b/worktree"}},
			dir:       "/a/b/worktree",
			wantEntry: "/a/b/worktree",
			wantOK:    true,
		},
		{
			name:   "string prefix without path boundary does not cover",
			wl:     direnvWhitelist{Prefix: []string{"/a/b"}},
			dir:    "/a/bc",
			wantOK: false,
		},
	}

	// whitelist.exact entries are not consulted at all — direnv resolves an
	// exact entry to a single RC file (a bare directory entry is normalized
	// to <dir>/.envrc), which can never cover the worktrees beneath dir. The
	// file-level semantics are pinned in TestSetupDirenvWhitelist
	// ("exact entries never satisfy the worktree prefix check").

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := whitelistCovers(tt.wl, tt.dir)
			if ok != tt.wantOK || entry != tt.wantEntry {
				t.Errorf("whitelistCovers(%+v, %q) = (%q, %v), want (%q, %v)",
					tt.wl, tt.dir, entry, ok, tt.wantEntry, tt.wantOK)
			}
		})
	}
}

func TestParseDirenvWhitelist(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		wl, err := parseDirenvWhitelist("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(wl.Prefix) != 0 {
			t.Errorf("expected empty whitelist, got %+v", wl)
		}
	})

	t.Run("multiline arrays and unknown sections", func(t *testing.T) {
		content := `[global]
warn_timeout = "1m"

[whitelist]
prefix = [
  "/home/x/github/truvity",
  "/home/x/gitlab",
]
exact = ["/home/x/one-off/.envrc"]
`
		wl, err := parseDirenvWhitelist(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(wl.Prefix) != 2 || wl.Prefix[0] != "/home/x/github/truvity" {
			t.Errorf("unexpected prefix entries: %v", wl.Prefix)
		}
	})

	t.Run("invalid toml", func(t *testing.T) {
		if _, err := parseDirenvWhitelist("[whitelist\nprefix = ["); err == nil {
			t.Error("expected error for invalid TOML")
		}
	})
}

func TestAddPrefixToWhitelist(t *testing.T) {
	tests := []struct {
		name    string
		content string
		entry   string
		// wantContains must appear verbatim in the updated content —
		// comments and untouched sections survive the textual edit.
		wantContains []string
		// wantPrefixLen, when non-zero, is the expected number of
		// whitelist.prefix entries after the add.
		wantPrefixLen int
	}{
		{
			name:    "empty file",
			content: "",
			entry:   "/a/b/worktree",
		},
		{
			name:    "no whitelist section",
			content: "[global]\nwarn_timeout = \"1m\"\n",
			entry:   "/a/b/worktree",
		},
		{
			name:          "section without prefix key",
			content:       "[whitelist]\nexact = [\"/one/.envrc\"]\n",
			entry:         "/a/b/worktree",
			wantContains:  []string{"exact = [\"/one/.envrc\"]"},
			wantPrefixLen: 1,
		},
		{
			name:          "single-line array",
			content:       "[whitelist]\nprefix = [\"/other\"]\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 2,
		},
		{
			name:          "single-line empty array",
			content:       "[whitelist]\nprefix = []\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 1,
		},
		{
			name:          "single-line array with trailing comma",
			content:       "[whitelist]\nprefix = [\"/other\",]\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 2,
		},
		{
			name:          "multiline array",
			content:       "[whitelist]\nprefix = [\n  \"/other\",\n  \"/another\",\n]\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 3,
		},
		{
			name:          "header with trailing comment",
			content:       "[whitelist]  # allow list\nprefix = [\"/other\"]\n",
			entry:         "/a/b/worktree",
			wantContains:  []string{"[whitelist]  # allow list"},
			wantPrefixLen: 2,
		},
		{
			name:          "single-line array with bracket in trailing comment",
			content:       "[whitelist]\nprefix = [\"/other\"] # note: ] here\n",
			entry:         "/a/b/worktree",
			wantContains:  []string{"# note: ] here"},
			wantPrefixLen: 2,
		},
		{
			name:          "multiline array with bracket in opener comment",
			content:       "[whitelist]\nprefix = [ # worktrees ]\n  \"/other\",\n]\n",
			entry:         "/a/b/worktree",
			wantContains:  []string{"# worktrees ]"},
			wantPrefixLen: 2,
		},
		{
			name:          "single-line array with bracket inside a value",
			content:       "[whitelist]\nprefix = [\"/weird]path\"]\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 2,
		},
		{
			name:          "single-line array with literal-string value",
			content:       "[whitelist]\nprefix = ['/other']\n",
			entry:         "/a/b/worktree",
			wantPrefixLen: 2,
		},
		{
			name:          "prefix key in later section is not touched",
			content:       "[whitelist]\nexact = []\n\n[other]\nprefix = [\"/decoy\"]\n",
			entry:         "/a/b/worktree",
			wantContains:  []string{"[other]\nprefix = [\"/decoy\"]"},
			wantPrefixLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updated := addPrefixToWhitelist(tt.content, tt.entry)

			wl, err := parseDirenvWhitelist(updated)
			if err != nil {
				t.Fatalf("updated content is not valid TOML: %v\n%s", err, updated)
			}

			if entry, ok := whitelistCovers(wl, tt.entry); !ok || entry != tt.entry {
				t.Errorf("entry %q not present after add, whitelist: %+v\n%s", tt.entry, wl, updated)
			}

			if tt.wantPrefixLen != 0 && len(wl.Prefix) != tt.wantPrefixLen {
				t.Errorf("expected %d prefix entries, got %v\n%s", tt.wantPrefixLen, wl.Prefix, updated)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(updated, want) {
					t.Errorf("updated content lost %q:\n%s", want, updated)
				}
			}

			// Pre-existing entries must survive the edit.
			origWl, err := parseDirenvWhitelist(tt.content)
			if err != nil {
				t.Fatalf("unexpected error parsing original: %v", err)
			}
			for _, p := range origWl.Prefix {
				found := false
				for _, q := range wl.Prefix {
					if p == q {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("pre-existing prefix %q lost after add:\n%s", p, updated)
				}
			}
		})
	}
}

func TestSetupDirenvWhitelist(t *testing.T) {
	target := "/repo/bar/worktree"

	t.Run("creates config when missing", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv", "direnv.toml")

		changed, err := setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected config to be created")
		}

		if err := checkDirenvWhitelist(configPath, target); err != nil {
			t.Errorf("check should pass after setup: %v", err)
		}
	})

	t.Run("no-op when a parent prefix already covers the target", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		original := "[whitelist]\nprefix = [\"/repo\"]\n"
		if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Error("expected no-op when parent prefix covers target")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != original {
			t.Errorf("config modified on no-op:\n%s", data)
		}
	})

	t.Run("idempotent across repeated runs", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")

		changed, err := setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("first run: %v", err)
		}
		if !changed {
			t.Error("first run should modify config")
		}

		afterFirst, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}

		changed, err = setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("second run: %v", err)
		}
		if changed {
			t.Error("second run should be a no-op")
		}

		afterSecond, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(afterFirst) != string(afterSecond) {
			t.Errorf("config changed on second run:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
		}
	})

	t.Run("adds to existing multiline whitelist without duplicating covered entries", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		original := "# my config\n[whitelist]\nprefix = [\n  \"/elsewhere\",\n]\n"
		if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected config to change")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}

		wl, err := parseDirenvWhitelist(string(data))
		if err != nil {
			t.Fatalf("result not valid TOML: %v\n%s", err, data)
		}
		if len(wl.Prefix) != 2 {
			t.Errorf("expected 2 prefix entries, got %v", wl.Prefix)
		}
		if got := string(data); !strings.Contains(got, "# my config") {
			t.Errorf("comment lost:\n%s", got)
		}
	})

	t.Run("check fails without coverage and hints the moon task", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		if err := os.WriteFile(configPath, []byte("[whitelist]\nprefix = [\"/elsewhere\"]\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := checkDirenvWhitelist(configPath, target)
		if err == nil {
			t.Fatal("expected check to fail")
		}
		if !strings.Contains(err.Error(), setupHint) {
			t.Errorf("error should hint %q, got: %v", setupHint, err)
		}
	})

	t.Run("exact entries never satisfy the worktree prefix check", func(t *testing.T) {
		// direnv resolves a whitelist.exact entry to a single RC file (a bare
		// directory entry is normalized to <dir>/.envrc, a /.env entry allows
		// only that .env file) and matches the loaded RC path exactly. None
		// of these forms make direnv allow <target>/<worktree>/.envrc, so
		// none of them may pass the check.
		content := "[whitelist]\nexact = [\n" +
			"  \"" + target + "\",\n" + // → <target>/.envrc only
			"  \"" + target + "/.envrc\",\n" +
			"  \"" + target + "/.env\",\n" +
			"  \"/elsewhere/.envrc\",\n" + // matches nothing under target
			"]\n"
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := checkDirenvWhitelist(configPath, target); err == nil {
			t.Error("exact entries must not satisfy the worktree prefix check")
		}
	})

	t.Run("setup adds a prefix even when exact entries name the target", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		original := "[whitelist]\nexact = [\"" + target + "\", \"" + target + "/.env\"]\n"
		if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := setupDirenvWhitelist(configPath, target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Error("expected setup to add a prefix entry despite exact entries")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		// Pre-existing exact entries must survive the edit verbatim.
		if !strings.Contains(string(data), "exact = [\""+target+"\", \""+target+"/.env\"]") {
			t.Errorf("exact entries lost after setup:\n%s", data)
		}

		if err := checkDirenvWhitelist(configPath, target); err != nil {
			t.Errorf("check should pass after setup: %v", err)
		}
	})

	t.Run("invalid existing TOML fails and leaves the file untouched", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		original := "[whitelist\nprefix = ["
		if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := setupDirenvWhitelist(configPath, target)
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
		if changed {
			t.Error("changed must be false on failure")
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != original {
			t.Errorf("file modified on failure:\noriginal:\n%s\ngot:\n%s", original, data)
		}
	})

	t.Run("edit failing re-validation leaves the file untouched", func(t *testing.T) {
		// [whitelist] exists only as an inline table: the textual editor
		// cannot find the header, appends a new [whitelist] section, and the
		// re-parse must reject the duplicate table before anything is written.
		configPath := filepath.Join(t.TempDir(), "direnv.toml")
		original := "whitelist = { prefix = [\"/elsewhere\"] }\n"
		if err := os.WriteFile(configPath, []byte(original), 0o644); err != nil {
			t.Fatal(err)
		}

		changed, err := setupDirenvWhitelist(configPath, target)
		if err == nil {
			t.Fatal("expected re-validation to fail for inline whitelist table")
		}
		if changed {
			t.Error("changed must be false on failure")
		}
		if !strings.Contains(err.Error(), "manually") {
			t.Errorf("error should point at the manual fallback, got: %v", err)
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != original {
			t.Errorf("file modified on failure:\noriginal:\n%s\ngot:\n%s", original, data)
		}
	})
}

func TestIsWhitelistTableHeader(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"[whitelist]", true},
		{"  [whitelist]  ", true},
		{"[whitelist] # allow list", true},
		{"[whitelist]# allow list", true},
		{"[whitelist.sub]", false},
		{"[whitelists]", false},
		{"# [whitelist]", false},
		{"prefix = []", false},
		{"[whitelist] trailing garbage", false},
	}

	for _, tt := range tests {
		if got := isWhitelistTableHeader(tt.line); got != tt.want {
			t.Errorf("isWhitelistTableHeader(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestTomlArrayCloseIndex(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int // -1: no close on this line; otherwise index of "]"
	}{
		{"plain close", `prefix = ["/a"]`, 14},
		{"close inside comment does not count", `prefix = [ # ]`, -1},
		{"bracket in string is skipped", `prefix = ["/a]b"]`, 16},
		{"bracket in literal string is skipped", `prefix = ['/a]b']`, 16},
		{"escaped quote in string", `prefix = ["/a\"]b"]`, 18},
		{"trailing comment after close", `prefix = ["/a"] # note ] here`, 14},
		{"unterminated array", `prefix = [`, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			open := strings.Index(tt.line, "[")
			if got := tomlArrayCloseIndex(tt.line, open); got != tt.want {
				t.Errorf("tomlArrayCloseIndex(%q, %d) = %d, want %d", tt.line, open, got, tt.want)
			}
		})
	}
}

// A dotfile-managed direnv.toml is a symlink into a repo; setup must
// update the LINKED file and keep the link, never replace the link with
// a regular file (which would orphan the dotfile source).
func TestSetupDirenvWhitelistFollowsSymlink(t *testing.T) {
	dotfiles := t.TempDir()
	realPath := filepath.Join(dotfiles, "direnv.toml")

	if err := os.WriteFile(realPath, []byte("[whitelist]\nprefix = [\"/elsewhere\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(t.TempDir(), "direnv.toml")
	if err := os.Symlink(realPath, link); err != nil {
		t.Fatal(err)
	}

	changed, err := setupDirenvWhitelist(link, "/home/u/bar/worktree")
	if err != nil || !changed {
		t.Fatalf("setup: changed=%v err=%v", changed, err)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat the link: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink must survive the atomic write, got mode %v", fi.Mode())
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(got), "/home/u/bar/worktree") || !strings.Contains(string(got), "/elsewhere") {
		t.Fatalf("linked dotfile source must carry the change and keep entries, got:\n%s", got)
	}
}
