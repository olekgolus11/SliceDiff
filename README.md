# SliceDiff

<img width="986" height="208" alt="image" src="https://github.com/user-attachments/assets/3607fd70-38b6-4e2a-8f93-a4202c227a64" />


SliceDiff is a local terminal UI for understanding large GitHub pull requests by turning raw file-by-file diffs into reviewable slices while keeping every grouped hunk traceable back to the original diff.

## Current State

SliceDiff is an active prototype that is usable locally today. It is built for reviewers who already have a large pull request in front of them and want a faster way to understand how related edits fit together.

<img width="1466" height="923" alt="image" src="https://github.com/user-attachments/assets/8f724daa-85d0-4345-bb7c-935265ed1450" />


This is not a GitHub review bot, not a production collaboration platform, and not a replacement for GitHub as the final place where review decisions happen. The current focus is local navigation, understanding, and experimentation with semantic diff grouping.

## What Works Today

- Load exactly one GitHub pull request through the GitHub CLI.
- Parse the unified diff into files, hunks, and line-level changes.
- Navigate the raw diff in a keyboard-first Bubbletea TUI.
- Optionally ask Codex or opencode to group hunks into semantic review slices.
- Switch between grouped slices and raw file-by-file diff order when slices are available.
- Regenerate AI slices for the current PR head SHA.
- Cache local slice results so repeated review sessions can reopen faster.
- Fall back to raw diff navigation when AI is disabled, unavailable, declined, or fails.

## What SliceDiff Does Not Do

- It does not post review comments.
- It does not approve pull requests.
- It does not request changes.
- It does not merge, close, or otherwise mutate GitHub pull request state.
- It does not call model APIs directly.
- It does not store GitHub, Codex, opencode, or model-provider credentials in repository-managed files.

## Tech Stack

- Go
- Bubbletea
- Lipgloss
- Chroma
- GitHub CLI (`gh`)
- Optional Codex or opencode CLI runners for AI grouping

## Run Locally

### Prerequisites

- Go 1.25 or newer
- GitHub CLI authenticated with access to the target PR

```sh
gh auth status
gh auth login
```

Optional AI runner:

```sh
codex login status
codex login
```

or:

```sh
opencode auth list
opencode auth login
```

### Start With Raw Diff Mode

From the repo root:

```sh
go run ./cmd/slicediff --no-ai olekgolus11/SliceDiff#1
```

Supported PR target formats:

```sh
go run ./cmd/slicediff --no-ai https://github.com/owner/repo/pull/123
go run ./cmd/slicediff --no-ai owner/repo#123
go run ./cmd/slicediff --no-ai 123
```

The plain number form must be run inside a local checkout that `gh repo view` can resolve.

### Start With AI Grouping

Run with Codex grouping:

```sh
go run ./cmd/slicediff --ai-runner codex --regen-slices owner/repo#123
```

Run with opencode grouping:

```sh
go run ./cmd/slicediff --ai-runner opencode --regen-slices owner/repo#123
```

### Build A Local Binary

```sh
go build -o slicediff ./cmd/slicediff
./slicediff --no-ai owner/repo#123
```

### Flags

- `--ai-runner codex|opencode`: override the saved AI runner for this run.
- `--no-ai`: skip AI consent, runner checks, cached slices, and agent invocation.
- `--regen-slices`: ignore cached slices and rerun the selected agent for the current PR head SHA.

## Keybindings

- `tab`: cycle panels.
- `j/k` or arrow keys: move selection.
- `pgup/pgdn`: scroll the focused details or hunk panel.
- `home/end`: jump to the start or end of the focused panel.
- `enter`: move deeper into the current review path.
- `v`: toggle grouped and raw views when slices are available.
- `r`: regenerate slices when AI is enabled.
- `?`: show or hide help.
- `q`: quit.

## Privacy And Local State

SliceDiff delegates GitHub authentication and access entirely to `gh`. It does not store GitHub tokens.

When AI grouping is enabled, SliceDiff delegates model authentication and model selection entirely to the selected runner. Codex and opencode own their own credentials, configuration, and provider behavior.

Before the first AI grouping request, SliceDiff asks for consent because PR metadata and diff hunks may be sent to the selected runner. If consent is declined, or if `--no-ai` is used, SliceDiff keeps the review local to raw diff navigation.

Local config lives at:

```sh
$(os.UserConfigDir)/slicediff/config.yaml
```

Local cache data lives under:

```sh
$(os.UserCacheDir)/slicediff
```

Config currently tracks the selected AI runner and AI consent state. Cache data may include fetched PR data and generated slice results for local reuse.

## Architecture Overview

- `cmd/slicediff`: CLI entrypoint and startup flags.
- `internal/github`: PR target parsing and `gh` integration.
- `internal/diff`: unified diff parsing and diff data types.
- `internal/agent`: runner checks, prompt generation, subprocess execution, and slice validation.
- `internal/config`: local config and cache handling.
- `internal/tui`: Bubbletea model, update loop, view rendering, styles, and tests.

The app keeps `main.go` thin and pushes behavior into package-level code so the raw diff path remains usable even when AI grouping is unavailable.

## Roadmap

- Add real screenshots or a terminal demo once the TUI has a polished public capture.
- Provide installable builds or releases instead of requiring `go run`.
- Improve grouped slice navigation and explanation quality.
- Harden Codex and opencode runner behavior across CLI version changes.
- Add clearer controls for resetting runner choice, consent, and local cache data.

## Status

SliceDiff is shared publicly for visibility, local experimentation, and feedback on the idea of semantic PR navigation. It should be read as a polished prototype, not as a finished product or a hosted review service.
