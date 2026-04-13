# SliceDiff

SliceDiff is a local terminal UI for understanding large GitHub pull requests. It fetches one PR through the GitHub CLI, parses the raw diff, and can ask Codex or opencode to group hunks into semantic review slices.

The app is focused on navigation and understanding. It does not post review comments, approve PRs, request changes, merge branches, or modify GitHub state.

## Prerequisites

- Go 1.23 or newer
- GitHub CLI authenticated with access to the target PR:

```sh
gh auth status
gh auth login
```

- Optional AI runner:

```sh
codex login status
codex login
```

or:

```sh
opencode auth list
opencode auth login
```

## Run

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

Run with Codex grouping:

```sh
go run ./cmd/slicediff --ai-runner codex --regen-slices owner/repo#123
```

Run with opencode grouping:

```sh
go run ./cmd/slicediff --ai-runner opencode --regen-slices owner/repo#123
```

Build a local binary:

```sh
go build -o slicediff ./cmd/slicediff
./slicediff --no-ai owner/repo#123
```

## Flags

- `--ai-runner codex|opencode`: override the saved AI runner for this run.
- `--no-ai`: skip AI consent, runner checks, cache lookup for slices, and agent invocation.
- `--regen-slices`: ignore cached slices and rerun the selected agent for the current PR head SHA.

## Keybindings

- `tab`: cycle focused panel.
- `j/k` or arrow keys: move selection.
- `pgup/pgdn`: scroll the focused details or hunk panel by a page.
- `home/end`: jump to the start or end of the focused panel.
- `enter`: drill focus to the next panel.
- `v`: toggle grouped and raw diff views when slices are available.
- `r`: regenerate slices when AI is enabled.
- `?`: toggle help text.
- `q`: quit.

## Troubleshooting

- GitHub auth error: run `gh auth login`.
- Missing Codex: install Codex or run with `--ai-runner opencode` or `--no-ai`.
- Codex auth error: run `codex login`.
- Missing opencode: install opencode or run with `--ai-runner codex` or `--no-ai`.
- opencode auth error: run `opencode auth login`.
- AI schema or malformed JSON error: retry with `--regen-slices`; use `--no-ai` to keep reviewing the raw diff.
- Timeout: retry with `r` in the app or run with `--no-ai`.

## Manual QA

Before sharing a build, run:

```sh
go test ./...
```

Then manually verify:

- Small PR raw mode: `go run ./cmd/slicediff --no-ai owner/repo#number`.
- Small PR Codex mode: `go run ./cmd/slicediff --ai-runner codex --regen-slices owner/repo#number`.
- Larger PR raw mode: confirm file and hunk lists scroll.
- Larger PR grouped mode: confirm slice, details, and hunk panels scroll independently.
- Missing GitHub auth: confirm the app shows `gh auth login` recovery text.
- Missing or unauthenticated runner: confirm the app falls back to raw diff with recovery text.
- AI failure: confirm the raw diff remains usable and details panel contains the full diagnostic.

## MVP Limits

- One GitHub PR per run.
- GitHub access is delegated entirely to `gh`.
- AI grouping supports Codex and opencode only.
- SliceDiff does not call model APIs directly and does not store model-provider credentials.
- SliceDiff does not mutate repository files or GitHub PR state.
