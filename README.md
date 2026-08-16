# workstation

Developer machine provisioning for the Truvity estate.

These are the setup steps **devbox, moon and proto structurally cannot do**:
those manage a *project's* environment, and this configures the *developer's
machine*. The boundary is that simple — is this about making an environment
ready, or about a product?

| Tool | Configures |
| --- | --- |
| `dockerctl` | `~/.docker/config.json` ECR credential helpers |

Still living in `truvity/bar` and moving here: home direnv whitelist, AWS SSO
session, goreleaser-pro licence + install, and the devbox↔npm playwright
lockstep.

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
