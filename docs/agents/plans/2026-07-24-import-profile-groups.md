---
date: 2026-07-24T18:32:30.387698+00:00
git_commit: 0f97d85897ddf55e19efe47dab4e0f966ed98ad0
branch: main
topic: "Import agent profile groups during profile creation"
tags: [plan, cli, tui, profiles, import]
status: ready
---

# PLAN: Import agent profile groups during profile creation

Add an optional import flow when creating a new `agentenv` profile. After entering the new profile name, users can choose an import source from existing profiles or the original host `HOME`, then select importable groups found in that source. The first implementation supports all recognized agent homes/groups for Pi, Claude Code, OpenCode, and Codex, regardless of which agent is being launched.

## Acceptance Criteria

- New profile creation first asks for and validates the profile name.
- After a valid new profile name, the TUI asks for an import source.
- Import sources include every existing `agentenv` profile home whose home directory exists plus the original process `HOME` when it exists.
- Users can choose no import.
- After selecting an import source, the TUI shows a multi-select list containing only groups whose source file or directory exists.
- Import groups cover recognized Pi, Claude Code, OpenCode, and Codex profile resources.
- Selected groups are copied into the new profile `HOME` before launching the agent.
- Existing target files/directories are skipped, never overwritten, and skipped paths are reported.
- If no importable groups exist in a selectable source, the flow clearly reports that and allows creating without import.
- Existing select-existing-profile behavior remains unchanged.
- Non-interactive behavior remains unchanged: no TUI import prompts are opened when `AGENTENV_NONINTERACTIVE=1`.
- Unit tests cover source discovery, group availability filtering, copy behavior, skip-existing conflicts, and CLI integration.
- TUI tests cover source and group selection views.
- README documents the import flow, supported groups, and skip-existing safety behavior.

## Technical Key Decisions and Tradeoffs

1. **Import scope:** Show all recognized agent groups for Pi, Claude Code, OpenCode, and Codex.
   - Why: User chose full multi-agent profile creation over current-agent-only import.
   - Impact: The import registry must model agent-specific roots and preserve enough metadata to render grouped labels.

2. **Conflict policy:** Skip existing target paths.
   - Why: User chose safest default that avoids deleting or overwriting target profile data.
   - Impact: Copy code must check target existence before each top-level group path and return a structured skipped list.

3. **Import timing:** Copy after `cfg.AddProfile` validation and after ensuring target profile home exists, before launching the selected agent.
   - Why: The selected profile must have a concrete profile HOME before files can be copied.
   - Impact: `chooseAndSaveProfile` should return import intent, while `run` performs filesystem import after resolving `paths.Paths`.

4. **Initial import granularity:** Groups map to complete files/directories, not individual files inside a group.
   - Why: Keeps the first implementation understandable and prevents complex merge semantics.
   - Impact: Selecting `pi skills` copies the whole `~/.pi/agent/skills` directory if the target path does not already exist.

5. **Source paths:** Existing profiles are read from `$AGENTENV_HOME/profiles/<profile>/home`; original HOME is read from the `HOME` environment variable captured before launching the child agent.
   - Why: This matches agentenv's profile-home model and the user requirement to offer the original HOME location.
   - Impact: Tests must set both `AGENTENV_HOME` and `HOME` to isolate discovery. Sources whose root does not exist are hidden.

6. **Copy safety:** Preserve symlinks as symlinks, do not follow them, and stop on the first unexpected copy error without rollback.
   - Why: Following symlinks can copy data outside the selected source HOME; rollback adds complexity and can delete unrelated target files.
   - Impact: The copy engine needs explicit symlink handling and partial-copy error tests.

7. **Import failure persistence:** On unexpected import failure, do not save the new profile or project mapping, but leave any already-created profile home/partial copied files in place and report the error.
   - Why: Avoids persisting a config entry for an incomplete profile while also avoiding risky rollback deletion.
   - Impact: CLI tests must assert config is unchanged and partial files may remain after failure.

8. **Group path type matching:** File groups require regular files and directory groups require real directories; top-level symlinks are hidden as kind mismatches.
   - Why: Showing top-level symlinked groups undermines file/dir validation and may create surprising dangling links. Symlinks inside selected directories are still preserved as symlinks.
   - Impact: Availability filtering uses `os.Stat`/`os.Lstat` carefully and tests cover top-level symlink mismatch plus nested symlink preservation.

9. **Multi-select defaults and cancel:** Group multi-select starts with all visible groups selected; `esc`/`ctrl+c` aborts the whole profile selection wizard.
   - Why: Importing from a chosen source usually means “copy useful state,” and existing TUI escape semantics cancel selection.
   - Impact: TUI tests need default selection and cancel behavior coverage.

## Current State

`agentenv` profiles currently have names only in config. Profile homes are just directories created on demand.

```text
agentenv run [--select] <agent> [args...]
      │
      ▼
internal/cli.App.run
      │
      ├─ paths.Resolve()
      ├─ config.Load()
      ├─ normalize cwd as project key
      ├─ profile := cfg.Projects[project]
      │
      ├─ missing mapping or --select
      │    └─ chooseAndSaveProfile(cfgPath, &cfg, project, agent)
      │       ├─ tui.ProfilePrompter.ChooseProfile(agent, cfg.Profiles)
      │       ├─ cfg.AddProfile(name) when created
      │       ├─ cfg.SetProject(project, name)
      │       └─ config.Save(cfgPath, cfg)
      │
      ├─ paths.EnsureProfileHome(p, profile)
      ├─ runner.LookupAgent(agent, p.BinDir())
      ├─ print banner
      └─ runner.RunAgent(real, pass, home, IO)
```

Key files:

- `internal/cli/cli.go` - profile selection/create/save and launch flow.
- `internal/tui/tui.go` - Bubble Tea profile selector and create-profile input mode.
- `internal/paths/paths.go` - `ProfileDir`, `ProfileHome`, and `EnsureProfileHome` helpers.
- `internal/config/config.go` - profile name validation and config persistence.
- `internal/cli/cli_test.go` - fake prompter seam for create/select flows.
- `internal/tui/tui_test.go` - snapshot-style TUI rendering tests.
- `test/e2e/run_test.go` - built-binary run behavior and non-interactive checks.
- `README.md` - user-facing profile behavior documentation.

Current create UI:

```text
╭─ agentenv ───────────────────────────────────────────────╮
│  • pi                                                    │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Create a Profile                                        │
│  Allowed: lowercase, numbers, dot, dash, underscore      │
│                                                          │
│  > profile-name                                          │
│                                                          │
│  enter create • esc/ctrl+c cancel                        │
╰──────────────────────────────────────────────────────────╯
```

## Desired End State

Creating a profile becomes a short wizard:

```text
Select/create profile
      │
      ├─ selected existing profile ────────────────► save mapping, run agent
      │
      └─ create new profile
            │
            ▼
         enter profile name
            │
            ▼
         select import source
            ├─ No import
            ├─ Original HOME: /Users/me
            ├─ profile: work
            └─ profile: personal
            │
            ▼
         select groups to import
            ├─ [x] pi / auth
            ├─ [x] pi / skills
            ├─ [ ] claude / agents
            ├─ [ ] opencode / plugins
            └─ [ ] codex / config
            │
            ▼
         create profile, copy selected missing paths, save mapping, run agent
```

Proposed source selector mockup:

```text
╭─ agentenv ───────────────────────────────────────────────╮
│ new-profile • pi                                         │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Import from                                             │
│  Choose optional source for profile files                │
│                                                          │
│  ▸ No import                                             │
│    Original HOME  /Users/florian                         │
│    profile: work  .../profiles/work/home                 │
│    profile: personal  .../profiles/personal/home         │
│                                                          │
│  ↑/↓/j/k move • enter select • esc/ctrl+c cancel         │
╰──────────────────────────────────────────────────────────╯
```

Proposed group multi-select mockup:

```text
╭─ agentenv ───────────────────────────────────────────────╮
│ new-profile • pi                                         │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Select groups to import                                 │
│  Source: Original HOME                                   │
│                                                          │
│  ▸ [x] pi auth        ~/.pi/agent/auth.json              │
│    [x] pi skills      ~/.pi/agent/skills/                │
│    [ ] pi extensions  ~/.pi/agent/extensions/            │
│    [ ] claude agents  ~/.claude/agents/                  │
│    [ ] opencode auth  ~/.local/share/opencode/auth.json  │
│    [ ] codex config   ~/.codex/config.toml               │
│                                                          │
│  space toggle • enter import • a all • n none • esc cancel│
╰──────────────────────────────────────────────────────────╯
```

Recognized group catalog for the first implementation:

```text
pi
  HOME/.pi/agent/AGENTS.md             -> .pi/agent/AGENTS.md
  HOME/.pi/agent/auth.json              -> .pi/agent/auth.json
  HOME/.pi/agent/settings.json          -> .pi/agent/settings.json
  HOME/.pi/agent/models.json            -> .pi/agent/models.json
  HOME/.pi/agent/trust.json             -> .pi/agent/trust.json
  HOME/.pi/agent/keybindings.json       -> .pi/agent/keybindings.json
  HOME/.pi/agent/extensions/            -> .pi/agent/extensions/
  HOME/.pi/agent/skills/                -> .pi/agent/skills/
  HOME/.pi/agent/prompts/               -> .pi/agent/prompts/
  HOME/.pi/agent/themes/                -> .pi/agent/themes/
  HOME/.pi/agent/git/                   -> .pi/agent/git/
  HOME/.pi/agent/npm/                   -> .pi/agent/npm/

claude
  HOME/.claude.json                     -> .claude/.claude.json
  HOME/.claude/.claude.json             -> .claude/.claude.json
  HOME/.claude/.credentials.json        -> .claude/.credentials.json
  HOME/.claude/settings.json            -> .claude/settings.json
  HOME/.claude/CLAUDE.md                -> .claude/CLAUDE.md
  HOME/.claude/agents/                  -> .claude/agents/
  HOME/.claude/skills/                  -> .claude/skills/
  HOME/.claude/commands/                -> .claude/commands/
  HOME/.claude/hooks/                   -> .claude/hooks/
  HOME/.claude/plugins/                 -> .claude/plugins/
  HOME/.claude/themes/                  -> .claude/themes/

opencode
  HOME/.config/opencode/opencode.json   -> .config/opencode/opencode.json
  HOME/.config/opencode/tui.json        -> .config/opencode/tui.json
  HOME/.config/opencode/AGENTS.md       -> .config/opencode/AGENTS.md
  HOME/.config/opencode/agents/         -> .config/opencode/agents/
  HOME/.config/opencode/commands/       -> .config/opencode/commands/
  HOME/.config/opencode/plugins/        -> .config/opencode/plugins/
  HOME/.config/opencode/skills/         -> .config/opencode/skills/
  HOME/.config/opencode/themes/         -> .config/opencode/themes/
  HOME/.config/opencode/tools/          -> .config/opencode/tools/
  HOME/.config/opencode/modes/          -> .config/opencode/modes/
  HOME/.local/share/opencode/auth.json  -> .local/share/opencode/auth.json
  HOME/.local/state/opencode/           -> .local/state/opencode/

codex
  HOME/.codex/config.toml               -> .codex/config.toml
  HOME/.codex/auth.json                 -> .codex/auth.json
  HOME/.codex/history.jsonl             -> .codex/history.jsonl
  HOME/.codex/skills/                   -> .codex/skills/
```

## Abstractions and Code Reuse

Add a small import package instead of embedding filesystem rules into `cli` or `tui`.

- `internal/profileimport/`
  - `catalog.go` - static group registry and labels.
    - `Group` - ID, agent, label, source relative path, target relative path, kind (`file` or `dir`).
    - `Catalog()` - returns recognized groups in stable display order.
  - `discover.go` - source and availability discovery.
    - `Source` - ID, label, path, kind (`home` or `profile`).
    - `Intent` - selected `Source` plus selected group IDs; copy resolves IDs against the current catalog.
    - `ProfileSources(paths.Paths, []config.Profile, originalHome string)` - existing profiles + original HOME, excluding missing/non-directory roots and deduplicating equal roots.
    - `AvailableGroups(source Source, groups []Group)` - filters to existing source paths with matching file/dir kind.
  - `copy.go` - safe copy engine.
    - `ImportSelection(targetHome string, intent Intent, catalog []Group) Result`.
    - `Result` - copied paths, skipped paths, first error.
- `internal/tui/tui.go`
  - Extend `ProfilePrompter` or add a new `ProfileCreatePrompter` interface that can return import intent.
  - Add Bubble Tea modes/models for import source and group multi-select.
- `internal/cli/cli.go`
  - Keep existing select-existing-profile behavior.
  - For profile creation, call TUI wizard, add profile, ensure target profile home, run import, save mapping, then launch.
- `internal/cli/cli_test.go`
  - Extend fake prompter to return profile creation plus import intent.
- `README.md`
  - Document optional import and supported groups.

Prefer new return types over boolean tuples once import metadata is added. A candidate interface:

```go
type ProfileChoice struct {
    Profile string
    Create  bool
    Import  *profileimport.Intent
}

type ProfilePrompter interface {
    ChooseProfile(agent string, profiles []config.Profile, sources []profileimport.Source, groups []profileimport.Group) (ProfileChoice, error)
}
```

`profileimport.Intent` should store selected group IDs rather than full `Group` values so fake prompters and tests do not duplicate catalog metadata. The copy engine resolves IDs against the catalog and returns a clear error for unknown IDs.

If backwards-compatible fake test updates become noisy, introduce a second interface method only for creation/import and adapt `BubblePrompter` internally.

## Logging & Observability

No persistent logs are required. Successful imports should print concise summaries to stdout before the banner or as part of normal command output.

Example:

```text
imported 4 group(s) from Original HOME into profile "customer-a"
skipped existing: .pi/agent/settings.json
```

Errors should include source and target path context:

```text
agentenv: import pi skills: copy /source/.pi/agent/skills to /target/.pi/agent/skills: permission denied
```

## Implementation

### Phase 1: Add import catalog and safe copy engine

Dependencies: None.

Create reusable, agent-agnostic import primitives with deterministic behavior and no TUI dependencies.

**Tasks**:
- [x] Create `internal/profileimport/catalog.go` with a stable catalog of Pi, Claude Code, OpenCode, and Codex import groups.
- [x] Represent each group with ID, agent, display label, source relative path, target relative path, and required path kind (`file` or `dir`).
- [x] Include all first-release groups listed in Desired End State.
- [x] Create `internal/profileimport/discover.go` with source modeling for original HOME and existing profile homes.
- [x] Define `profileimport.Intent` with source identity/path and selected group IDs only.
- [x] Hide source roots that are missing or not directories, including missing original HOME and profile homes not created yet; do not hide existing roots merely because they contain zero importable groups.
- [x] Deduplicate import sources by cleaned absolute root path while preserving display order: Original HOME first, then profiles sorted/config order.
- [x] Implement group availability filtering by checking both source existence and required file/dir kind; top-level symlink groups are hidden as kind mismatches.
- [x] Create `internal/profileimport/copy.go` with recursive copy support for files and directories.
- [x] Preserve symlinks as symlinks and never follow symlinks while copying.
- [x] Preserve file and directory permissions as much as Go's standard library reasonably supports.
- [x] Ensure target parent directories are created with `0700`; copied top-level directories may preserve source mode after the private parent exists.
- [x] Implement skip-existing policy: if the top-level target path exists, do not copy or merge that group and add it to `Result.Skipped`.
- [x] Stop on the first unexpected copy error, return copied/skipped paths accumulated so far, and do not roll back partial copies.
- [x] Return structured copied/skipped details and contextual errors.

**Automated Verification**:
- [x] `go test ./internal/profileimport` passes.
- [x] Unit test: catalog contains expected groups and stable IDs.
- [x] Unit test: `AvailableGroups` returns only groups whose source file/dir exists and matches required kind.
- [x] Unit test: top-level symlink group paths are hidden as kind mismatches.
- [x] Unit test: source discovery hides missing or not-yet-created source roots, but keeps existing roots with zero importable groups.
- [x] Unit test: source discovery orders Original HOME before profiles and deduplicates equal roots.
- [x] Unit test: copying a selected file creates parent directories and preserves content.
- [x] Unit test: copying a selected directory recursively copies nested files.
- [x] Unit test: symlinks are copied as symlinks and not followed.
- [x] Unit test: existing target path is skipped and source content is not copied over it.
- [x] Unit test: unexpected copy error stops the import without rollback and reports copied/skipped paths accumulated so far.
- [x] Unit test: unknown group IDs in an import intent produce a clear error.
- [x] Unit test: missing selected source path returns a clear error according to the implemented API contract.

### Phase 2: Integrate import intent into profile creation flow

Dependencies: Phase 1.

Wire the new import engine into `agentenv run` while preserving existing behavior for existing profile selection and non-interactive runs.

**Tasks**:
- [x] Update `internal/tui.ProfilePrompter` return type or add a companion choice type so profile creation can return optional import intent.
- [x] Add a temporary `BubblePrompter` implementation path that returns no import until Phase 3 wizard UI is complete, so interactive creation remains functional during this phase.
- [x] Update fake prompters in `internal/cli/cli_test.go` to support import intent without opening Bubble Tea.
- [x] Refactor `internal/cli.App.run` / `chooseAndSaveProfile` so profile creation can ensure the profile home and perform import before launch.
- [x] Build import sources from current config profiles plus original `HOME` using `profileimport.ProfileSources`.
- [x] Exclude the newly-created profile from import sources until it exists, and keep existing source profile list stable.
- [x] When importing from an existing profile, use `$AGENTENV_HOME/profiles/<source>/home` as source root.
- [x] When importing from original HOME, use `os.Getenv("HOME")` from the agentenv process.
- [x] Save config and project mapping only after successful profile add/import flow; on import failure, leave any partial profile home files in place but do not persist the new profile or project mapping.
- [x] Print a concise import summary when copied or skipped groups exist.
- [x] Preserve `AGENTENV_NONINTERACTIVE=1` behavior: no prompts, no profile home creation, no import attempt when prompting would be required.
- [x] Ensure selecting an existing profile does not trigger source/group import dialogs.

**Automated Verification**:
- [x] `go test ./internal/cli` passes.
- [x] Unit test: creating a profile with no import behaves like current creation and launches with empty new profile HOME.
- [x] Unit test: creating with original HOME import copies selected existing groups into the new profile HOME.
- [x] Unit test: creating with existing profile import copies selected groups from that profile home.
- [x] Unit test: skipped existing target paths are reported and not overwritten.
- [x] Unit test: unexpected import failure leaves config/mapping unchanged while allowing partial profile home files to remain.
- [x] Unit test: selecting an existing profile does not call import logic.
- [x] Unit test: non-interactive missing mapping still fails without creating a profile home or importing.

### Phase 3: Add TUI source selector and group multi-select wizard

Dependencies: Phase 1 and Phase 2.

Implement the interactive dialogs that gather import source and group selections during new profile creation.

**Tasks**:
- [x] Extend `internal/tui/tui.go` profile creation model with post-name import steps.
- [x] Add source selection mode showing `No import`, `Original HOME`, and existing profile sources.
- [x] Add group multi-select mode after selecting a source with available groups.
- [x] Default all visible import groups to selected when entering group multi-select mode.
- [x] Filter the group list to only groups where source file/directory exists and matches the catalog kind.
- [x] If a selected source has zero available groups, show a clear message and allow proceeding without import.
- [x] Add keyboard handling for multi-select: `space` toggles, `enter` confirms, `a` selects all, `n` selects none, arrows/j/k move, esc/ctrl+c cancel.
- [x] Make `esc`/`ctrl+c` in import source or group modes abort the whole profile selection wizard, consistent with existing selection/create behavior.
- [x] Keep existing create-profile validation error behavior for invalid names.
- [x] Keep existing select-profile and remove-profile UI unchanged except for any necessary type updates.
- [x] Update render helpers if wider group labels need truncation or wrapping.

**Automated Verification**:
- [x] `go test ./internal/tui` passes.
- [x] TUI snapshot test: import source selection renders sources and help text.
- [x] TUI snapshot test: group multi-select renders only available groups.
- [x] TUI update test: `space` toggles the selected group.
- [x] TUI update test: all visible groups are selected by default when entering group multi-select mode.
- [x] TUI update test: `a` selects all and `n` clears all.
- [x] TUI update test: confirming no import returns creation choice with nil import intent.
- [x] TUI update test: `esc`/`ctrl+c` in import source and group modes cancels the whole wizard.
- [x] Existing TUI snapshot tests are updated intentionally and still pass.

**Manual Verification**:
- [ ] In an unmapped project, run `agentenv run pi`, choose create profile, enter a name, select Original HOME, select at least one import group, and confirm the profile launches.
- [ ] Confirm the created profile HOME contains the selected copied paths and not unselected paths.
- [ ] Repeat with a source profile and confirm files copy from that profile's HOME.
- [ ] Create a target conflict manually, import the same group, and confirm the target is not overwritten and the skipped path is reported.

### Phase 4: E2E coverage and documentation

Dependencies: Phase 2 and Phase 3.

Cover built-binary invariants that do not require terminal key automation and update user docs.

**Tasks**:
- [x] Add E2E helpers or unit-level seams as needed to avoid brittle Bubble Tea key automation for the successful wizard path.
- [x] Add E2E or CLI-level test that non-interactive missing mapping still fails without import side effects.
- [x] Add E2E or CLI-level test around an already-mapped project to prove normal run behavior is unchanged after import code lands.
- [x] Update `README.md` command/profile description to mention optional imports on profile creation.
- [x] Document import sources: existing profiles and original HOME.
- [x] Document supported group families: Pi, Claude Code, OpenCode, Codex, with concise concrete examples of files/directories for each family.
- [x] Document skip-existing behavior and that import is a top-level group copy, not a merge.
- [x] Keep README concise and avoid listing secrets values; mention auth files only as files that may be copied when selected.

**Automated Verification**:
- [x] `mise run fmt` succeeds.
- [x] `mise run test` succeeds.
- [x] `mise run test:e2e` succeeds.
- [x] `mise run build` succeeds.

**Manual Verification**:
- [ ] Read the README import section and confirm it accurately matches the implemented TUI flow and safety behavior.

## Implementation Notes

- Implemented `internal/profileimport` catalog, source discovery, availability filtering, and safe copy engine.
- Added Claude Code state/credential import groups and set `CLAUDE_CONFIG_DIR=$HOME/.claude` for Claude runs to support isolated accounts.
- Fixed Claude state import for `CLAUDE_CONFIG_DIR`: Claude writes state to `$CLAUDE_CONFIG_DIR/.claude.json`, so legacy `~/.claude.json` is imported to `.claude/.claude.json`.
- Set `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, and `XDG_STATE_HOME` for all agent runs so imported XDG config/data/state are read from the isolated profile even when host XDG variables are set.
- Added OpenCode state import for `.local/state/opencode/`; OpenCode stores selected TUI theme in `kv.json` there.
- Integrated import intent into profile creation; config save is delayed until import succeeds.
- Added TUI source selection, group multi-select, and empty-source handling.
- Automated verification passed; manual TUI/README verification remains for the user.

## References

- `internal/cli/cli.go` - current profile creation, mapping, non-interactive handling, and launch flow.
- `internal/tui/tui.go` - current Bubble Tea profile selector/create model.
- `internal/paths/paths.go` - profile HOME path helpers.
- `internal/config/config.go` - profile validation and config persistence.
- `internal/cli/cli_test.go` - fake profile prompter test seam.
- `internal/tui/tui_test.go` - existing TUI snapshot pattern.
- `test/e2e/run_test.go` - E2E run behavior tests.
- `README.md` - user-facing command and runtime file documentation.
- Pi docs: `/opt/homebrew/lib/node_modules/@earendil-works/pi-coding-agent/README.md`, `docs/extensions.md`, `docs/skills.md`, `docs/prompt-templates.md`, `docs/themes.md`, `docs/models.md`, `docs/providers.md`, `docs/settings.md`, `docs/packages.md`.
- Claude Code docs researched: `https://code.claude.com/docs/en/settings.md`, `memory.md`, `agents.md`, `skills.md`, `plugins.md`, `hooks.md`.
- OpenCode docs researched: `https://raw.githubusercontent.com/sst/opencode/dev/packages/web/src/content/docs/{config,agents,commands,plugins,skills,themes,providers,rules,mcp-servers}.mdx`.
- Codex docs/source researched: `https://raw.githubusercontent.com/openai/codex/main/README.md`, `docs/{config,authentication,skills,slash_commands,agents_md}.md`, and `codex-rs/core/config.schema.json`.
