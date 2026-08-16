// Command direnvctl keeps direnv's whitelist covering a directory.
//
// direnv refuses to load an .envrc it has not been told to trust. For a
// directory that holds many working copies — git worktrees, checkouts — the
// answer is a whitelist.prefix entry covering the parent, so every copy
// beneath it is trusted without a per-copy `direnv allow`.
//
//	direnvctl check <dir>    # is <dir> covered?
//	direnvctl setup <dir>    # cover it, if nothing already does
//
// Ported from truvity/bar's retired barctl. One thing changed: bar derived
// the directory from its own git layout (<main repo root>/worktree), which
// tied the code to one repository. The directory is an argument now — the
// caller knows which one it wants, this tool knows how direnv decides
// coverage.
//
// The edit is textual, so comments and formatting in direnv.toml survive, and
// it is re-parsed before the file is replaced. Symlinked configs are followed
// rather than overwritten: a dotfiles setup keeps working.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/truvity/workstation/internal/direnvcfg"
)

// main stays a thin wrapper so os.Exit never strands a deferred cancel:
// os.Exit skips defers, so the two cannot share a function.
func main() {
	if err := cli(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func cli() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	return run(ctx, logger, os.Args[1:])
}

func run(ctx context.Context, logger *slog.Logger, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: direnvctl (check|setup) <dir>")
	}

	// Absolute, because direnv matches on absolute RC paths — a relative
	// entry would silently never match anything.
	dir, err := filepath.Abs(args[1])
	if err != nil {
		return fmt.Errorf("resolve %s: %w", args[1], err)
	}

	switch args[0] {
	case "check":
		return direnvcfg.Check(ctx, logger, dir)
	case "setup":
		return direnvcfg.Setup(ctx, logger, dir)
	default:
		return fmt.Errorf("unknown command %q (want: check, setup)", args[0])
	}
}
