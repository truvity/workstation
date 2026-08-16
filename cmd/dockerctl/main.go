// Command dockerctl points Docker at the ECR credential helper for the
// accounts you name.
//
// The accounts are ARGUMENTS, not built in: this repo owns the mechanism
// and the calling repo owns which accounts it needs. That split is why
// this could leave bar at all — bar's platform.yaml keeps the data.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/truvity/workstation/internal/dockercfg"
)

type ecrFlag []string

func (e *ecrFlag) String() string { return strings.Join(*e, ",") }

func (e *ecrFlag) Set(v string) error {
	account, region, ok := strings.Cut(v, ":")
	if !ok || account == "" || region == "" {
		return fmt.Errorf("want <account-id>:<region>, got %q", v)
	}

	*e = append(*e, fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", account, region))

	return nil
}

func main() {
	var ecr ecrFlag

	check := flag.Bool("check", false, "report whether the helpers are configured; write nothing")
	flag.Var(&ecr, "ecr", "ECR account as <account-id>:<region> (repeatable)")
	flag.Parse()

	if err := run(ecr, *check); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(hosts []string, check bool) error {
	if len(hosts) == 0 {
		return fmt.Errorf("no --ecr given; nothing to configure")
	}

	path, err := dockercfg.Path()
	if err != nil {
		return err
	}

	cfg, err := dockercfg.Read(path)
	if err != nil {
		return err
	}

	if _, ok := dockercfg.Helpers(cfg); !ok {
		fmt.Fprintln(os.Stderr,
			"warning: credHelpers in "+path+" is not an object; it will be replaced")
	}

	missing := dockercfg.Missing(cfg, hosts)

	// --check is the same code path as the write, deliberately. A
	// separate validator is how the checker and the fixer drift until
	// one reports healthy about something the other would change.
	if check {
		if len(missing) > 0 {
			return fmt.Errorf("not configured for %s (run without --check to fix)",
				strings.Join(missing, ", "))
		}

		fmt.Printf("✓ docker credential helpers configured for %d registr%s\n",
			len(hosts), plural(len(hosts)))

		return nil
	}

	// Checked only on the write path: --check must stay usable on a
	// machine that has not installed the helper yet, which is exactly
	// the machine whose setup is being verified.
	if _, err := exec.LookPath("docker-credential-ecr-login"); err != nil {
		return fmt.Errorf("docker-credential-ecr-login not found in PATH "+
			"(install: devbox global add amazon-ecr-credential-helper): %w", err)
	}

	if len(missing) == 0 {
		fmt.Printf("✓ already configured for %d registr%s\n", len(hosts), plural(len(hosts)))

		return nil
	}

	changed := dockercfg.Apply(cfg, hosts)

	if err := dockercfg.WriteAtomic(path, cfg); err != nil {
		return err
	}

	fmt.Printf("✓ configured %s in %s\n", strings.Join(changed, ", "), path)

	return nil
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}

	return "ies"
}
