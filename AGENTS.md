# AGENTS.md

Instructions for AI coding agents working with `workstation`, in this repo or
in a repo that consumes it. Human-readable too — nothing here is agent-only.

## What this repo ships

Five binaries. `workstation` is the **repository** name, never a command:

| Command | Does |
| --- | --- |
| `dockerctl` | `~/.docker/config.json` ECR credential helpers |
| `awsctl` | keeps an AWS SSO session alive (idempotent) |
| `licencectl` | fetches + caches the goreleaser-pro licence |
| `playwrightctl` | keeps the nix browsers and the npm runner in step |
| `direnvctl` | keeps direnv's whitelist covering a directory of working copies |

Writing `workstation dockerctl …` in a shell fails with
`workstation: command not found`.

## The command surface, exhaustively

This is the whole of it. There are no other subcommands and no other flags:

```
dockerctl     [--check] --ecr <account-id>:<region> [--ecr <account-id>:<region> …]
awsctl
licencectl    [--secret-id <id>] [--profile <aws>] [--region <r>] [--force] [--path]
playwrightctl [check | latest]
direnvctl     (check | setup) <dir>
```

Four shapes that are easy to guess wrong:

- **`awsctl` takes no flags and no arguments.** Selecting a profile is
  `AWS_PROFILE=… awsctl`. bar's docs claimed a `--profile` flag for a year;
  the binary never parsed one and silently ignored it.
- **`--ecr` is repeatable**, one per registry, and its value is a single
  `account:region` pair — not two arguments.
- **`direnvctl` requires its `<dir>`**; there is no default and no cwd
  fallback. `direnvctl setup` alone is a usage error.
- **`playwrightctl` defaults to `check`** when given no argument.

Source of truth: the `cmd/<tool>/main.go` files. If this file and those
disagree, those are right and this one is a bug — fix it.

## If you are about to write a command into documentation

Read `cmd/<tool>/main.go` first. Not the README, not a grep of how often a
string appears in other docs, and not another repo's prose.

This warning exists because of a real incident (2026-08-16). An agent cleaning
up references to a retired tool turned a repo-level fact into a command-level
invocation — wrong binary, wrong argument shape — and wrote it into several
files as fact. It then "verified" the fix by grepping documentation, which only
measured how often the mistake had been repeated.

A plausible command that fails is worse than an admission that you do not know
the command. If you cannot verify an invocation, write that it is unverified.

## This repository is public, and that is a constraint

These tools take an account id, a region, a secret id and a profile — and every
one of them is a **caller argument**, never a constant in this repo. That is
the whole reason a machine-provisioning repo can be public.

`hack/leak-canary.sh` enforces it in CI (recipe: `just leak-canary`). Adding a
real account id, ARN, ECR hostname, bucket or internal hostname — *including in
a README example* — fails the build. Examples use `<account-id>` placeholders
for exactly this reason. If you need a new pattern, add it to the canary; never
add an exception without one.

## Using these tools from another repo

Do not `go install` and do not add them to `devbox.json` packages. Pin in
`go.mod` with a `tool` directive, add a `bin/<tool>` wrapper that `exec`s
`go run`, and put `bin/` on PATH via devbox.

One trap, learned the hard way in bar: `bin/.gitignore` there is deny-by-default
(`*` plus an explicit `!name` per wrapper). A new wrapper that is not listed
works on the machine that created it and is `command not found` in CI.

## Working in this repo

- `just check` runs the lot (build, test, lint, vuln, leak-canary).
- Tools take their data as arguments. A tool that reads a project config file
  or hardcodes a coordinate belongs in the calling repo, not here.
- Home-directory writes go through `internal/atomicfile`, which resolves
  symlinks first — these files are frequently symlinks into a dotfiles repo.
