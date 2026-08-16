// Command playwrightctl keeps the two halves of playwright in step.
//
// playwright is one thing wearing two hats: an npm package (the runner) and a
// nix package (the browsers). They must be the same version. The npm side can
// move to a release nixpkgs has never packaged, and the mismatch surfaces at
// RUN time as a driver error — not at install, which is why it is worth a
// check of its own.
//
// Ported from truvity/bar's retired barctl (the playwright half of
// setup-env-check), with the missing half added: bar could only ever say
// "these disagree", never "here is the newest version you can actually have".
//
//	playwrightctl check     # do the two sides agree?
//	playwrightctl latest    # highest version BOTH sides can supply
//
// `latest` is what makes an upgrade decidable. nixpkgs lags npm, so the
// answer is the newest release present in both — bumping npm past it is the
// mistake this exists to prevent.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

var semver = regexp.MustCompile(`\d+\.\d+\.\d+`)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cmd := "check"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error

	switch cmd {
	case "check":
		err = check(ctx)
	case "latest":
		err = latest(ctx)
	default:
		err = fmt.Errorf("unknown command %q (want: check, latest)", cmd)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func check(ctx context.Context) error {
	nix, err := nixVersion(ctx)
	if err != nil {
		return err
	}

	npm, err := npmVersion(ctx)
	if err != nil {
		return err
	}

	if nix != npm {
		return fmt.Errorf("playwright mismatch: nix has %s, npm has %s.\n"+
			"They ship together — the nix package provides the browsers the npm runner drives.\n"+
			"Run `playwrightctl latest` for the newest version both sides can supply, and move both.",
			nix, npm)
	}

	fmt.Printf("✓ playwright %s on both sides\n", nix)

	return nil
}

func latest(ctx context.Context) error {
	nixAll, err := nixAvailable(ctx)
	if err != nil {
		return err
	}

	npmAll, err := npmAvailable(ctx)
	if err != nil {
		return err
	}

	have := make(map[string]bool, len(npmAll))
	for _, v := range npmAll {
		have[v] = true
	}

	var common []string

	for _, v := range nixAll {
		if have[v] {
			common = append(common, v)
		}
	}

	if len(common) == 0 {
		return fmt.Errorf("no version present in both nixpkgs and npm — that should not happen; check both lists by hand")
	}

	sort.Slice(common, func(i, j int) bool { return less(common[j], common[i]) })

	fmt.Println(common[0])

	return nil
}

// nixVersion asks the installed browsers what they are.
func nixVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "playwright", "--version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("playwright --version: %w (is playwright-test installed via devbox?)", err)
	}

	v := semver.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("no version in %q", strings.TrimSpace(string(out)))
	}

	return v, nil
}

// npmVersion reads the version the workspace actually resolved.
func npmVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "yarn", "why", "playwright", "--json").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("yarn why playwright: %w (is playwright installed in a workspace?)", err)
	}

	v := semver.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("no version in yarn why output")
	}

	return v, nil
}

func nixAvailable(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "devbox", "search", "playwright-test", "--show-all").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("devbox search playwright-test: %w", err)
	}

	return semver.FindAllString(string(out), -1), nil
}

func npmAvailable(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "npm", "view", "playwright", "versions", "--json").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("npm view playwright versions: %w", err)
	}

	var versions []string
	if err := json.Unmarshal(out, &versions); err != nil {
		return nil, fmt.Errorf("parse npm versions: %w", err)
	}

	return versions, nil
}

// less compares dotted semver numerically — string order puts 1.9.0 above
// 1.10.0, which would recommend an older release as "latest".
func less(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])

		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x < y
		}
	}

	return len(as) < len(bs)
}
