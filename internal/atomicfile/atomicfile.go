// Package atomicfile writes a file so a reader never sees half of it.
//
// Both config writers here edit files someone else depends on — Docker's
// config and direnv's — where a partial write is not a lost edit but a
// corrupt file the tool refuses to parse. Temp file in the same directory,
// chmod, write, sync, close, rename.
//
// Symlinks are resolved first, deliberately: dotfile managers routinely make
// ~/.config/direnv/direnv.toml a link into a dotfiles repo, and renaming over
// the link would replace it with a regular file, silently detaching the user
// from their own configuration.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces path with data, atomically.
func Write(path string, data []byte, dirPerm, filePerm os.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolve %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
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
