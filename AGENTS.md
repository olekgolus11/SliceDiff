# AGENTS.md

This repository contains **SliceDiff**, a Go Bubbletea terminal app for reviewing a single GitHub pull request locally. The app fetches PR metadata and diffs through `gh`, optionally groups hunks with `codex` or `opencode`, and renders the result as a keyboard-first TUI.

## What The App Does

- Accepts exactly one PR target per run.
- Resolves PRs through the GitHub CLI only.
- Parses unified diffs into files, hunks, and line-level changes.
- Optionally delegates semantic grouping to an external runner.
- Renders grouped or raw diff navigation in the terminal.
- Stores only local config and cache data.

## Repository Layout

- `cmd/slicediff/main.go` - CLI entrypoint and flag parsing.
- `internal/github` - PR target parsing plus `gh` integration.
- `internal/diff` - unified diff parser and diff data types.
- `internal/agent` - runner readiness checks, prompt generation, subprocess execution, and slice validation.
- `internal/config` - local YAML config and cache handling.
- `internal/tui` - Bubbletea model, update loop, view, styles, and tests.

## Important Boundaries

- Do not add code that posts review comments, approves, requests changes, merges, or mutates GitHub state.
- Do not call model APIs directly from the app; AI grouping is delegated to CLI runners only.
- Do not store GitHub, Codex, opencode, or provider credentials in repo-managed files.
- Keep the raw diff path usable when AI is unavailable, declined, or fails.
- Keep `main.go` thin; prefer pushing behavior into package-level code and tests.

## Local State

- Config lives at `$(os.UserConfigDir)/slicediff/config.yaml`.
- Cache lives under `$(os.UserCacheDir)/slicediff`.
- Config currently tracks the selected AI runner and AI consent state.

## Working Conventions

- Prefer small, focused changes in the package that owns the behavior.
- Preserve the existing fallback and error-copy behavior when touching auth, runner, or cache code.
- Use the Bubbletea skill/workflow when working on app behavior, TUI state, layout, input handling, or rendering.
- If making UI or visual changes, use the Frontend Design skill so the result stays intentional and polished.
- Keep the TUI keyboard-first and responsive to terminal resizing.
- When editing diff or agent logic, update tests alongside the code.
- Use ASCII by default unless the file already depends on non-ASCII text.
- Do not overwrite unrelated uncommitted changes in the worktree.

## Testing And Verification

Run the full test suite before handing off changes:

```sh
go test ./...
```

Useful manual checks:

```sh
go run ./cmd/slicediff --no-ai owner/repo#123
go run ./cmd/slicediff --ai-runner codex --regen-slices owner/repo#123
go run ./cmd/slicediff --ai-runner opencode --regen-slices owner/repo#123
```

For build verification:

```sh
go build ./cmd/slicediff
```

## Editing Tips

- Favor table-driven tests for parser, validation, and config behavior.
- Keep error messages concrete and user-facing where they surface in the TUI or CLI.
- When changing cache keys or prompt/schema versions, update both the code and any tests that assume the old values.
- Diff parsing is deliberately permissive, but hunk IDs and traceability must stay stable.
- If you change the shape of agent output, make sure validation still guarantees that every reported hunk maps back to a real diff hunk.
