# workstation

Developer machine provisioning for the Truvity estate.

These are the setup steps **devbox, moon and proto structurally cannot do**:
those manage a *project's* environment, and this configures the *developer's
machine*. The boundary is that simple — is this about making an environment
ready, or about a product?

| Tool | Does |
| --- | --- |
| `dockerctl` | `~/.docker/config.json` ECR credential helpers |
| `awsctl` | keeps an AWS SSO session alive — idempotent, so other steps can depend on it |
| `licencectl` | fetches the goreleaser-pro licence, cached **once per machine** |
| `playwrightctl` | keeps the nix browsers and the npm runner in step, and finds the newest shared version |

Still in `truvity/bar`: the home direnv whitelist.

### Two behaviours changed in the move, deliberately

**`licencectl` caches per machine, not per repo.** bar cached to
`<gitRoot>/bin/.goreleaser-key`, so every clone and every git worktree fetched
the same secret again. A licence belongs to the developer, not to a checkout.

**`playwrightctl` can answer, not just complain.** bar could say "these two
disagree" but never "here is the newest version you can actually have".
`playwrightctl latest` prints the highest release present in **both** nixpkgs
and npm — the number an upgrade needs, since npm regularly offers versions
nixpkgs has never packaged and the mismatch only surfaces at run time.

## Why a separate repo

All of this used to live in bar, inside the retired `barctl` package. That
placement was the problem: onboarding to gitops, gemaal or any other repo
meant checking out bar to configure your machine.

## Use

Tools take their *data* as arguments — which AWS accounts, which regions — so
this repo carries the mechanism and the calling repo carries the policy. Run
them without cloning anything:

```bash
go run github.com/truvity/workstation/cmd/dockerctl@v0.1.0 \
  --ecr 123456789012:eu-central-1 \
  --ecr 210987654321:eu-central-1
```

`--check` verifies without writing, so a repo's environment check and the
thing that fixes it are the same binary and cannot drift apart.

**Pin the version.** These tools write to your home directory; `@latest` is
the last place you want an unpinned fetch resolving differently per machine
and per day.
