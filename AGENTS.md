# Agent Instructions

## Project Overview

`agentenv` launches AI coding agents with project-specific identities by setting an isolated profile `HOME`.

- CLI entrypoint: `cmd/agentenv/main.go`
- CLI command handling: `internal/cli`
- config loading/saving: `internal/config`
- paths/profile homes: `internal/paths`
- wrapper/PATH setup: `internal/wrapper`, `internal/pathsetup`
- profile imports: `internal/profileimport`
- TUI flows: `internal/tui`
- e2e tests: `test/e2e`

Runtime locations:

- Config: `${AGENTENV_CONFIG_HOME:-user config dir}/agentenv/config.toml`
- Data root: `${AGENTENV_HOME:-$HOME/.local/share/agentenv}`
- Wrapper bin dir: `$AGENTENV_HOME/bin`
- Profile HOME: `$AGENTENV_HOME/profiles/<profile>/home`

## Development Commands

Prefer `mise` tasks:

```sh
mise install
mise run fmt
mise run test
mise run test:e2e
mise run build
```

Task meanings:

- `mise run fmt`: `gofmt -w cmd internal test`
- `mise run test`: `go test ./internal/...`
- `mise run test:e2e`: `go test ./test/e2e/...`
- `mise run build`: `go build -o agentenv ./cmd/agentenv`

## Required Verification

When changing Go code:

1. Run `mise run fmt`.
2. Run `mise run test`.

Also run `mise run test:e2e` when changes affect:

- CLI command behavior or output
- profile selection/creation/removal
- profile import behavior
- wrapper, unwrap, PATH setup, or shell integration
- install/release behavior
- environment isolation with `HOME`, `XDG_*`, `AGENTENV_HOME`, or `AGENTENV_CONFIG_HOME`
- TUI flows

Run `mise run build` before finishing user-visible CLI or release-related changes.

## Coding Guidance

- Keep command behavior backward-compatible unless explicitly asked otherwise.
- Add or update tests close to the affected package.
- Use e2e tests for full command behavior, subprocess launches, filesystem layout, and environment isolation.
- Tests must not touch real user config, data, or home directories. Use temp dirs and set `AGENTENV_CONFIG_HOME`, `AGENTENV_HOME`, and `HOME` explicitly when needed.
- `AGENTENV_NONINTERACTIVE=1` must not open TUI prompts.
- Profile imports copy whole selected groups. They do not merge contents. Existing target files/directories are skipped and must not be overwritten.
- `run` must resolve the real agent executable from `PATH` while avoiding recursion through the agentenv wrapper bin directory.
