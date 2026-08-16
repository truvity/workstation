// Command awsctl keeps an AWS SSO session alive.
//
// Idempotent by design: it checks first and only logs in when the session is
// actually gone, so it is safe to make every other setup step depend on it —
// which is what the caller's task graph does.
//
// Ported from truvity/bar's retired barctl (cmd/setup-login). It reads no
// project config and takes no profile: `aws sso login` uses the ambient AWS
// configuration, so selecting a profile is `AWS_PROFILE=… awsctl`, the
// standard way. bar's docs claimed a `--profile` flag for a year; the binary
// never parsed one and silently ignored it.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
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

	return run(ctx, logger)
}

func run(ctx context.Context, logger *slog.Logger) error {
	if sessionActive(ctx) {
		logger.InfoContext(ctx, "AWS SSO session active")

		return nil
	}

	logger.InfoContext(ctx, "AWS SSO session expired, running aws sso login")

	// Interactive: it opens a browser and waits, so it needs the real stdio.
	login := exec.CommandContext(ctx, "aws", "sso", "login")
	login.Stdin = os.Stdin
	login.Stdout = os.Stdout
	login.Stderr = os.Stderr

	if err := login.Run(); err != nil {
		return fmt.Errorf("aws sso login: %w", err)
	}

	// Verify rather than trust the exit code: `aws sso login` can succeed
	// against a misconfigured profile and leave you without credentials.
	if !sessionActive(ctx) {
		return fmt.Errorf("credentials still invalid after login — check your SSO configuration")
	}

	logger.InfoContext(ctx, "AWS SSO login successful")

	return nil
}

// sessionActive reports whether the current credentials resolve. Output is
// discarded: this is a probe, and its failure is the normal path.
func sessionActive(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "aws", "sts", "get-caller-identity")
	cmd.Stdout = nil
	cmd.Stderr = nil

	return cmd.Run() == nil
}
