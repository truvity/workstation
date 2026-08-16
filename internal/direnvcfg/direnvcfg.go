package direnvcfg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/truvity/workstation/internal/atomicfile"
)

const (
	// setupHint names the command that fixes the problem this package reports.
	// It is generic on purpose: this code no longer lives in the repo whose
	// task used to be the answer.
	setupHint = "direnvctl setup <dir>"
)

var (
	// whitelistPrefixKeyRegexp matches the `prefix = [` assignment inside the
	// [whitelist] section of direnv.toml.
	whitelistPrefixKeyRegexp = regexp.MustCompile(`^\s*prefix\s*=\s*\[`)
)

// direnvConfigPath returns the path to direnv.toml.
// Respects XDG_CONFIG_HOME, falls back to ~/.config/direnv/direnv.toml.
func direnvConfigPath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}

		configDir = filepath.Join(home, ".config")
	}

	return filepath.Join(configDir, "direnv", "direnv.toml"), nil
}

// readDirenvConfig reads the direnv.toml file. Returns empty string if not found.
func readDirenvConfig(configPath string) (string, error) {
	//nolint:gosec // G304: path is direnv config, not user-controlled
	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}

		return "", fmt.Errorf("read %s: %w", configPath, err)
	}

	return string(data), nil
}

type (
	// direnvWhitelist holds the whitelist.prefix entries of direnv.toml.
	// whitelist.exact is deliberately not modeled: an exact entry allows a
	// single RC file and can never cover the worktree prefix — see
	// whitelistCovers for the verified direnv semantics.
	direnvWhitelist struct {
		Prefix []string `toml:"prefix"`
	}

	// direnvConfig is the subset of direnv.toml we care about; unknown
	// keys/sections are ignored by the TOML decoder.
	direnvConfig struct {
		Whitelist direnvWhitelist `toml:"whitelist"`
	}
)

// parseDirenvWhitelist extracts whitelist.prefix entries from direnv.toml
// content.
func parseDirenvWhitelist(content string) (direnvWhitelist, error) {
	var cfg direnvConfig
	if _, err := toml.Decode(content, &cfg); err != nil {
		return direnvWhitelist{}, fmt.Errorf("parse direnv.toml: %w", err)
	}

	return cfg.Whitelist, nil
}

// pathPrefixCovers reports whether the whitelist prefix entry covers dir
// using path-boundary semantics: /a/b covers /a/b and /a/b/c, but not /a/bc.
// Both paths are cleaned before comparison. direnv itself uses a plain
// strings.HasPrefix, so anything we accept here direnv accepts too.
func pathPrefixCovers(entry, dir string) bool {
	entry = filepath.Clean(entry)
	dir = filepath.Clean(dir)

	if entry == dir {
		return true
	}

	// Root ("/") already ends with the separator; avoid doubling it.
	if !strings.HasSuffix(entry, string(filepath.Separator)) {
		entry += string(filepath.Separator)
	}

	return strings.HasPrefix(dir, entry)
}

// whitelistCovers reports whether the whitelist guarantees that direnv will
// auto-allow every .envrc beneath dir, returning the covering entry.
//
// Matching mirrors direnv's own semantics, verified against its source
// (internal/cmd/config.go whitelist loading and internal/cmd/rc.go
// RC.Allowed, direnv master as of 2026-08):
//
//   - whitelist.prefix: direnv allows an RC file when any prefix entry is a
//     plain string prefix of the RC file's absolute path
//     (strings.HasPrefix(rcPath, prefix)), so a prefix entry covering dir at
//     a path boundary covers dir/<worktree>/.envrc too. Our path-boundary
//     comparison accepts a strict subset of direnv's string match, so
//     anything accepted here is accepted by direnv as well.
//
//   - whitelist.exact: at config-load time direnv appends ".envrc" to every
//     entry that does not already end in "/.envrc" or "/.env", then requires
//     an exact match with the loaded RC file's absolute path. An exact entry
//     therefore allows exactly one file — "/dir" allows only "/dir/.envrc",
//     and "/dir/.env" allows only "/dir/.env", never "/dir/.envrc" — and can
//     never cover the open-ended set of <dir>/<worktree>/.envrc files
//     beneath the worktree prefix. Exact entries are consequently not
//     consulted here: treating one as coverage would pass the check while
//     direnv still blocks the worktrees.
func whitelistCovers(wl direnvWhitelist, dir string) (string, bool) {
	for _, p := range wl.Prefix {
		if pathPrefixCovers(p, dir) {
			return p, true
		}
	}

	return "", false
}

// checkDirenvWhitelist verifies that requiredDir is covered by the whitelist
// in the given direnv.toml file.
func checkDirenvWhitelist(configPath, requiredDir string) error {
	content, err := readDirenvConfig(configPath)
	if err != nil {
		return err
	}

	wl, err := parseDirenvWhitelist(content)
	if err != nil {
		return err
	}

	entry, ok := whitelistCovers(wl, requiredDir)
	if !ok {
		return fmt.Errorf("worktree prefix %s not covered by direnv whitelist (run: %s)", requiredDir, setupHint)
	}

	fmt.Printf("✓ direnv worktree whitelist configured (covered by %q)\n", entry)

	return nil
}

// CheckDirenvWorktreeWhitelist validates that the worktree prefix is covered
// by the direnv.toml whitelist. Any whitelist.prefix entry that is a
// path-prefix of <main repo root>/worktree satisfies the check — direnv's
// whitelist.prefix entries cover every directory beneath them.
func Check(ctx context.Context, logger *slog.Logger, requiredDir string) error {
	logger.InfoContext(ctx, "checking direnv whitelist", slog.String("dir", requiredDir))

	configPath, err := direnvConfigPath()
	if err != nil {
		return err
	}

	return checkDirenvWhitelist(configPath, requiredDir)
}

// The directory to cover is now a PARAMETER, not derived here. In bar this
// resolved <main repo root>/worktree through a git helper, which tied this
// code to one repository's layout and to a git dependency it did not
// otherwise need. The caller knows which directory it wants covered; this
// package knows how direnv decides coverage. That split is what let the code
// leave bar.

// SetupDirenvWorktreeWhitelist ensures the direnv whitelist covers the
// worktree prefix, adding the stable <main repo root>/worktree entry only
// when no existing entry already covers it.
func Setup(ctx context.Context, logger *slog.Logger, targetDir string) error {
	logger.InfoContext(ctx, "configuring direnv worktree whitelist")

	configPath, err := direnvConfigPath()
	if err != nil {
		return err
	}

	changed, err := setupDirenvWhitelist(configPath, targetDir)
	if err != nil {
		return err
	}

	if changed {
		fmt.Printf("✅ direnv worktree whitelist configured (added %q)\n", targetDir)
	} else {
		fmt.Println("✅ direnv worktree whitelist already configured")
	}

	return nil
}

// setupDirenvWhitelist is the testable core of SetupDirenvWorktreeWhitelist:
// it adds targetDir to whitelist.prefix in the config file at configPath
// unless an existing entry already covers it. Returns whether the file was
// modified. The edit is textual (comments and formatting are preserved) and
// validated by re-parsing before the file is replaced.
func setupDirenvWhitelist(configPath, targetDir string) (bool, error) {
	content, err := readDirenvConfig(configPath)
	if err != nil {
		return false, err
	}

	wl, err := parseDirenvWhitelist(content)
	if err != nil {
		return false, err
	}

	if _, ok := whitelistCovers(wl, targetDir); ok {
		return false, nil
	}

	updated := addPrefixToWhitelist(content, targetDir)

	// Validate the textual edit before touching the user's config.
	updatedWl, err := parseDirenvWhitelist(updated)
	if err != nil {
		return false, fmt.Errorf("updated config would be invalid, add %q to whitelist.prefix manually: %w", targetDir, err)
	}

	if _, ok := whitelistCovers(updatedWl, targetDir); !ok {
		return false, fmt.Errorf("failed to update config automatically, add %q to whitelist.prefix manually", targetDir)
	}

	// Atomic replace: a crash mid-write must never leave a truncated
	// direnv.toml behind.
	if err := atomicfile.Write(configPath, []byte(updated), 0o750, 0o600); err != nil {
		return false, fmt.Errorf("write %s: %w", configPath, err)
	}

	return true, nil
}

// isWhitelistTableHeader reports whether line is the [whitelist] table
// header, tolerating surrounding whitespace and a trailing comment
// ("[whitelist]  # allow list"). Sub-tables ("[whitelist.x]") and
// commented-out headers ("# [whitelist]") do not match.
func isWhitelistTableHeader(line string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), "[whitelist]")
	if !ok {
		return false
	}

	rest = strings.TrimSpace(rest)

	return rest == "" || strings.HasPrefix(rest, "#")
}

// tomlArrayCloseIndex returns the index of the "]" that closes the array
// opened at openIdx in line, or -1 when the array does not close on this
// line. The scan is TOML-aware: basic ("...", with backslash escapes) and
// literal ('...') strings are skipped, a "#" outside a string starts a
// comment that ends the scan, and nested "[...]" pairs are tracked. A plain
// strings.LastIndex would be fooled by a "]" inside a value or a trailing
// comment.
func tomlArrayCloseIndex(line string, openIdx int) int {
	depth := 0

	for i := openIdx; i < len(line); i++ {
		switch line[i] {
		case '"':
			for i++; i < len(line) && line[i] != '"'; i++ {
				if line[i] == '\\' {
					i++ // skip the escaped character
				}
			}
		case '\'':
			for i++; i < len(line) && line[i] != '\''; i++ { //nolint:revive // empty body: scan to closing quote
			}
		case '#':
			return -1
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

// addPrefixToWhitelist inserts entry into the whitelist.prefix array of the
// TOML content, preserving existing formatting. Handles a missing file/section,
// a [whitelist] section without a prefix key, and single-line or multi-line
// prefix arrays. The result is validated by the caller before being written.
func addPrefixToWhitelist(content, entry string) string {
	quoted := strconv.Quote(entry)

	lines := strings.Split(content, "\n")

	sectionStart := -1
	for i, line := range lines {
		if isWhitelistTableHeader(line) {
			sectionStart = i
			break
		}
	}

	if sectionStart == -1 {
		// No [whitelist] section: append one. If the section exists in a form
		// we cannot find textually (e.g. an inline table), the duplicate table
		// is rejected by the TOML re-parse in setupDirenvWhitelist.
		out := content
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}

		return out + "[whitelist]\nprefix = [" + quoted + "]\n"
	}

	// Find the prefix key within the section (before the next section header).
	for i := sectionStart + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") {
			break
		}

		if !whitelistPrefixKeyRegexp.MatchString(lines[i]) {
			continue
		}

		open := strings.Index(lines[i], "[")
		if closing := tomlArrayCloseIndex(lines[i], open); closing != -1 {
			// Single-line array: insert before the closing bracket.
			inside := strings.TrimSpace(lines[i][open+1 : closing])
			sep := ", "
			switch {
			case inside == "":
				sep = ""
			case strings.HasSuffix(inside, ","):
				sep = " "
			}
			lines[i] = lines[i][:closing] + sep + quoted + lines[i][closing:]
		} else {
			// Multi-line array: insert a new element right after the opener.
			lines = append(lines[:i+1], append([]string{"  " + quoted + ","}, lines[i+1:]...)...)
		}

		return strings.Join(lines, "\n")
	}

	// Section exists but has no prefix key: add one right after the header.
	lines = append(lines[:sectionStart+1], append([]string{"prefix = [" + quoted + "]"}, lines[sectionStart+1:]...)...)

	return strings.Join(lines, "\n")
}
