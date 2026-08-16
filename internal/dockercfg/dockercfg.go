// Package dockercfg edits ~/.docker/config.json credential helpers.
//
// Ported from truvity/bar's retired barctl package. Two properties came
// with it and must not be lost in any future rewrite:
//
//   - MERGE, never clobber. The file is read, credHelpers is merged into
//     it, and everything else — auths, credsStore, plugin settings, the
//     keys we know nothing about — is written back untouched. A rewrite
//     that marshals a fresh config silently destroys a developer's Docker
//     setup, and they find out at the next docker login.
//   - ATOMIC write. Temp file in the same directory, chmod, write, sync,
//     close, rename. A partial write here leaves Docker unable to parse
//     its own config.
package dockercfg

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	// filePerm matches what Docker itself writes.
	filePerm = 0o600
	// helper is the credential helper binary name, minus the
	// docker-credential- prefix Docker prepends.
	helper = "ecr-login"
)

// Path returns the Docker config location, honouring DOCKER_CONFIG.
func Path() (string, error) {
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return filepath.Join(dir, "config.json"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".docker", "config.json"), nil
}

// Read parses the Docker config, returning an empty map when absent.
func Read(path string) (map[string]any, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the Docker config, not user-controlled
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]any), nil
		}

		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	cfg := make(map[string]any)
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return cfg, nil
}

// Helpers returns the credHelpers map, or an empty one when the key is
// absent or holds something other than an object. A non-object is
// reported rather than silently replaced: it means someone hand-edited
// the file, and overwriting without a word is how you lose their work.
func Helpers(cfg map[string]any) (map[string]any, bool) {
	raw, ok := cfg["credHelpers"]
	if !ok {
		return make(map[string]any), true
	}

	m, ok := raw.(map[string]any)
	if !ok {
		return make(map[string]any), false
	}

	return m, true
}

// Missing returns the hosts that are not yet mapped to the ECR helper, in
// input order. Empty means the config is already correct — which is what
// --check reports on.
func Missing(cfg map[string]any, hosts []string) []string {
	current, _ := Helpers(cfg)

	var missing []string

	for _, host := range hosts {
		if got, ok := current[host].(string); !ok || got != helper {
			missing = append(missing, host)
		}
	}

	return missing
}

// Apply merges the ECR helper entries into cfg, returning the hosts it
// added or corrected.
func Apply(cfg map[string]any, hosts []string) []string {
	current, _ := Helpers(cfg)
	changed := Missing(cfg, hosts)

	for _, host := range hosts {
		current[host] = helper
	}

	cfg["credHelpers"] = current

	return changed
}

// WriteAtomic writes cfg via a temp file and rename, so a crash mid-write
// cannot leave Docker with a config it refuses to parse.
func WriteAtomic(path string, cfg map[string]any) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode docker config: %w", err)
	}

	data = append(data, '\n')

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	name := tmp.Name()
	defer os.Remove(name) //nolint:errcheck // best-effort cleanup on failure

	if err := tmp.Chmod(filePerm); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("chmod temp file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", name, path, err)
	}

	return nil
}

// SortedKeys is a small helper for deterministic output.
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}
