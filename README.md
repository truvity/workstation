# workstation

Developer machine provisioning for the Truvity estate.

| Tool | Does |
| --- | --- |
| `dockerctl` | `~/.docker/config.json` ECR credential helpers |
| `awsctl` | keeps an AWS SSO session alive — idempotent, so other steps can depend on it |
| `licencectl` | fetches the goreleaser-pro licence, cached **once per machine** |
| `playwrightctl` | keeps the nix browsers and the npm runner in step, and finds the newest shared version |
| `direnvctl` | keeps direnv's whitelist covering a directory of working copies |

Each tool is a standalone binary, published only as a tagged Go module —
there is no other distribution. Nothing is left in `truvity/bar` — barctl is
fully retired.

## Who it is for

Developers on Truvity estate machines who need machine-level setup done once
and kept correct — not project-level environment. These are the setup steps
**devbox, moon and proto structurally cannot do**: those manage a *project's*
environment, and this configures the *developer's machine*. The boundary is
that simple — is this about making an environment ready, or about a product?
`workstation` deliberately does not install a project's toolchain,
dependencies or environment variables; that stays with devbox, moon and
proto.

## The model

Two nouns: a **tool** and its **caller**. A tool takes its *data* — which AWS
accounts, which regions, which secret — as arguments, never as a constant in
this repo; the calling repo (or the developer, at the shell) supplies that
data, and this repo supplies the mechanism. That split is why this could
leave `bar` at all: everything here used to live inside the retired `barctl`
package there, and onboarding to gitops, gemaal or any other repo meant
checking out bar just to configure your machine.

### Two behaviours changed in the move, deliberately

**`licencectl` caches per machine, not per repo.** bar cached to
`<gitRoot>/bin/.goreleaser-key`, so every clone and every git worktree fetched
the same secret again. A licence belongs to the developer, not to a checkout.

**`playwrightctl` can answer, not just complain.** bar could say "these two
disagree" but never "here is the newest version you can actually have".
`playwrightctl latest` prints the highest release present in **both** nixpkgs
and npm — the number an upgrade needs, since npm regularly offers versions
nixpkgs has never packaged and the mismatch only surfaces at run time.

## Install and a worked example

Run a tool without cloning anything, pinned to a tagged version:

```bash
go run github.com/truvity/workstation/cmd/dockerctl@v0.1.2 \
  --ecr <account-id>:<region> \
  --ecr <account-id>:<region>
```

The placeholders are not politeness: `hack/leak-canary.sh` fails the build on
anything shaped like an account id, so the examples here *cannot* drift into
real coordinates. A caller supplies them — for example, a repo's own
onboarding script reading its own configuration and passing one `--ecr` per
registry.

`--check` verifies without writing, so a repo's environment check and the
thing that fixes it are the same binary and cannot drift apart.

**Pin the version.** These tools write to your home directory; `@latest` is
the last place you want an unpinned fetch resolving differently per machine
and per day.

## Consumers

Developer machines in the Truvity estate. These tools are called by:

- **bar** — workstation is the new home for machine-provisioning tools that
  bar calls during developer onboarding
- Each repository that needs machine setup — e.g., for ECR credentials or AWS
  SSO session management

## Neighbours

`workstation` overlaps with `access-roster` on developer-machine AWS
credentials:

- **access-roster** via `accessctl`: the estate path for minting AWS
  credentials with dynamic scope and audience gating, issued by the OIDC
  issuer and requiring no stored secrets
- **awsctl**: the SSO fallback when access-roster is unreachable; both live
  on the same machine and are called as alternatives

`ocictl` handles ECR authentication at build time. `workstation` via
`dockerctl` handles ECR authentication on a developer machine.

## Documentation

There is no separate `docs/` directory; the reference lives beside the code:

| Doc | Covers |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | the exhaustive command surface, one entry per tool, and the conventions for working in this repo |
| [`CHANGELOG.md`](CHANGELOG.md) | one heading per tag |
| `cmd/<tool>/main.go` | the doc comment on each command; AGENTS.md itself defers to these as the source of truth |

## The rule that makes this repository public

Every account id, region, secret id and profile these tools touch is a
**caller argument** — `--ecr`, `--secret-id`, `--profile`, `--region` — never
a constant committed here. That split is the whole reason a
machine-provisioning repo can be public.

[`hack/leak-canary.sh`](hack/leak-canary.sh) enforces it mechanically: it
scans tracked files for account ids, ARNs, ECR hosts, internal domains and
committed tokens, and the `check` recipe in the [`Justfile`](Justfile) runs
it — a match fails the build, not just a report.

## Status

All five tools are built and released: `dockerctl`, `awsctl`, `licencectl`,
`playwrightctl` and `direnvctl`. `v0.1.0` was the initial public release
after the move out of `bar`; the latest tag is `v0.1.2`. CI runs `build`,
`test`, `lint` and `leak-canary` on every push and pull request; a separate
daily workflow runs `vuln`. Nothing described in this README is pending.

## Development

`devbox shell` (or `devbox run -- <recipe>`) provides the pinned toolchain —
Go, golangci-lint, gopls, govulncheck, just. From there, the `Justfile`
recipes:

- `just build` — compiles every command (a compile check only: these tools
  are consumed via `go run`/`go tool`, never installed from this repo)
- `just test` — unit tests, with coverage
- `just lint` — `golangci-lint config verify`, then `run`
- `just vuln` — `govulncheck`
- `just leak-canary` — runs `hack/leak-canary.sh`
- `just check` — build, test, lint, vuln and leak-canary together
- `just tidy` — `go mod tidy`

There are no charts and no golden renders in this repository.

## Releasing

There is no release workflow and no build artifact beyond the tag itself —
these tools are consumed as source via `go run …@vX`, so cutting a release is
tagging one. `vars.AUTO_RELEASE` is unset for this repository: every tag,
patch included, is a person's tag, pushed by hand and published with
`gh release create`.

## Licence

MIT — see [`LICENSE`](LICENSE).
