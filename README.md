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

## What replaced barctl

`barctl` was one CLI doing four unrelated jobs. It is gone, and each job
went somewhere with a real boundary — this table is the map, because for a
while the answer lived only in people's heads and the docs kept pointing at
a binary nobody could run.

| barctl did | now |
| --- | --- |
| `barctl artifacts …` — charts, OCI packaging, digest pinning | **`ocictl`** (`helmctl`) |
| `barctl test --project …` — ephemeral test installs | **`gemaal`** / `gemaalctl` |
| `barctl release …` — image builds | GoReleaser, driven by moon |
| `barctl prepare …` — docker/direnv/login/licence, machine setup | **this repo** |

The split is the point. Registry and chart work is `ocictl`'s; ephemeral
installations are `gemaal`'s; building is the build tool's; and what is
left — the developer's own machine — is the only part none of them can
own, which is why this repo exists rather than a fifth pile in bar.

If you are reading a doc that still says `barctl`, it is stale. Find the
row above.
