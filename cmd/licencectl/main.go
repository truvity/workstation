// Command licencectl fetches the goreleaser-pro licence and caches it.
//
// Ported from truvity/bar's retired barctl (cmd/setup-goreleaser-key). One
// behaviour changed deliberately in the move: the cache is now USER-level, not
// repo-level.
//
// bar cached to <gitRoot>/bin/.goreleaser-key, so every clone and every git
// worktree fetched the licence again — the same secret, re-read from Secrets
// Manager, once per working copy. The licence belongs to the developer, not to
// a checkout, so it is cached once per machine and every repo reads it.
//
// Two sources, in order:
//
//   - GORELEASER_KEY in the environment. The CI face: a runner has no SSO
//     session, so the Secrets Manager path cannot work there.
//   - AWS Secrets Manager, via the ambient SSO session.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	fileName = "goreleaser-key"
	filePerm = 0o600
	dirPerm  = 0o700
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
	secretID := flag.String("secret-id", "", "AWS Secrets Manager secret id holding {\"apikey\": …}")
	profile := flag.String("profile", "", "AWS profile for the secret (optional; ambient credentials otherwise)")
	region := flag.String("region", "", "AWS region for the secret")
	force := flag.Bool("force", false, "re-fetch even when a cached licence exists")
	printPath := flag.Bool("path", false, "print the cache path and exit")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	path, err := cachePath()
	if err != nil {
		return err
	}

	if *printPath {
		fmt.Println(path)

		return nil
	}

	return run(ctx, logger, path, *secretID, *profile, *region, *force)
}

// cachePath is user-level and XDG-aware: one licence per machine.
func cachePath() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "truvity", fileName), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	return filepath.Join(home, ".config", "truvity", fileName), nil
}

func run(ctx context.Context, logger *slog.Logger, path, secretID, profile, region string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			logger.InfoContext(ctx, "licence already cached", slog.String("path", path))

			return nil
		}
	}

	key, source, err := fetch(ctx, secretID, profile, region)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(key), filePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	logger.InfoContext(ctx, "licence cached",
		slog.String("path", path), slog.String("source", source))

	return nil
}

func fetch(ctx context.Context, secretID, profile, region string) (key, source string, err error) {
	// CI first: a runner has the org secret in the environment and no SSO
	// session, so trying Secrets Manager there would fail slowly and
	// confusingly.
	if k := os.Getenv("GORELEASER_KEY"); k != "" {
		return k, "GORELEASER_KEY", nil
	}

	if secretID == "" {
		return "", "", fmt.Errorf("no GORELEASER_KEY in environment and no --secret-id given")
	}

	args := []string{"secretsmanager", "get-secret-value",
		"--secret-id", secretID, "--output", "json", "--query", "SecretString"}
	if profile != "" {
		args = append(args, "--profile", profile)
	}

	if region != "" {
		args = append(args, "--region", region)
	}

	out, err := exec.CommandContext(ctx, "aws", args...).CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("aws secretsmanager get-secret-value: %w (%s)", err, out)
	}

	// The CLI returns the secret as a quoted JSON string, so it unwraps twice.
	var quoted string
	if err := json.Unmarshal(out, &quoted); err != nil {
		return "", "", fmt.Errorf("parse secret string: %w", err)
	}

	var secret struct {
		APIKey string `json:"apikey"`
	}

	if err := json.Unmarshal([]byte(quoted), &secret); err != nil {
		return "", "", fmt.Errorf("parse secret json: %w", err)
	}

	if secret.APIKey == "" {
		return "", "", fmt.Errorf("secret %q has no apikey field", secretID)
	}

	return secret.APIKey, "secretsmanager:" + secretID, nil
}
