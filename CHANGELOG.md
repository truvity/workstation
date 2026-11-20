# Changelog

All notable changes to this project are documented here. For releases, see
the [releases page](https://github.com/truvity/workstation/releases).

## v0.2.0

### Removed

- `licencectl` and its goreleaser-pro licence fetch (a breaking removal, so a
  minor version). Nothing uses goreleaser Pro any more: every truvity release
  runs the free goreleaser, and the org secret `GORELEASER_KEY` was deleted on
  2026-11-19. README and AGENTS.md no longer describe it, and the tool count
  is four.

## v0.1.2

- README pins the install example to the latest tag and gains `Consumers` and `Neighbours`; `renovate.json` extends the shared preset (the `go` directive is owned by devbox-update); ci-workflows pins moved to v3.13.1.

## v0.1.1

### Fixed

- `playwrightctl latest` now works: stderr was being parsed as JSON instead
  of stdout
- Nine lint issues found on first run

## v0.1.0

### Added

- Initial public release
- Five binaries: `dockerctl`, `awsctl`, `licencectl`, `playwrightctl`,
  `direnvctl`
- `direnvctl` — the final tool moved out of the retired internal monorepo
