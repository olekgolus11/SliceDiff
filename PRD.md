# SliceDiff PRD

## 1. Executive Summary

### Problem Statement

AI-assisted development is increasing the size and complexity of GitHub pull requests, while reviewers still have to reconstruct intent from raw file-by-file diffs. Individual reviewers inspecting a large PR locally need a faster way to understand which files and hunks belong to the same feature, behavior change, or refactor without losing traceability to the original diff.

### Proposed Solution

SliceDiff is a Go Bubbletea TUI that targets a single GitHub PR through the `gh` CLI, retrieves its diff and metadata, and delegates AI grouping to an installed coding-agent CLI. In the MVP, SliceDiff supports Codex and opencode as non-interactive subprocess runners, then renders their semantic grouping output as reviewable slices with exact links back to raw hunks.

### Success Criteria

- Reduce reviewer "what is this doing?" comments on large PRs by at least 40% in user-reported before/after comparisons.
- At least 80% of beta users report that SliceDiff improves their time-to-understanding for PRs with 20+ changed files.
- AI-generated slices achieve at least 75% reviewer acceptance in manual evaluation, where acceptance means the reviewer agrees that the grouped hunks belong together.
- 100% of displayed semantic slices preserve traceability to exact file paths and diff hunks.
- The TUI can load and render a PR with 100 changed files and 5,000 changed lines within 10 seconds after `gh` returns the diff payload.

## 2. User Experience & Functionality

### User Personas

- Individual code reviewer: A developer asked to review a large GitHub PR locally and understand the change before leaving feedback in GitHub.
- Senior engineer or maintainer: A reviewer who needs to separate behavior changes, refactors, generated edits, and test updates before deciding how to inspect the PR.
- PR author doing a self-review: A developer who wants to understand whether their own PR reads coherently before requesting review.

### User Stories

- As an individual reviewer, I want to open a single GitHub PR from my terminal so that I can review it without leaving my local workflow.
- As an individual reviewer, I want SliceDiff to use my existing GitHub CLI authentication so that I do not manage another GitHub token.
- As an individual reviewer, I want to choose Codex or opencode as the AI runner so that SliceDiff fits my existing agent setup.
- As an individual reviewer, I want AI-generated semantic slices so that I can understand the PR by feature or intent instead of by file order.
- As an individual reviewer, I want each slice to explain why its hunks were grouped so that I can trust or challenge the grouping.
- As an individual reviewer, I want to drill from a slice to the exact raw diff hunks so that no implementation detail is hidden from me.
- As an individual reviewer, I want to switch between grouped and raw diff views so that I can recover the original GitHub ordering when needed.
- As an individual reviewer, I want keyboard-first navigation with optional mouse support so that the app works naturally in terminal review sessions.

### Authentication & First-Run Setup

- GitHub authentication:
  - SliceDiff requires the `gh` CLI for the MVP.
  - SliceDiff validates GitHub access with `gh auth status` before loading a PR.
  - If `gh` is missing, unauthenticated, or cannot access the PR, SliceDiff blocks PR loading and shows recovery guidance, including `gh auth login`.
  - SliceDiff does not implement app-owned GitHub OAuth, store personal access tokens, or call the GitHub API directly in the MVP.

- AI runner setup:
  - SliceDiff supports only `codex` and `opencode` as MVP AI runners.
  - On first AI-enabled run, if no runner is configured, SliceDiff shows a TUI picker for Codex or opencode and saves the selected runner in config.
  - Subsequent runs use the saved runner unless the user passes a startup override such as `--ai-runner codex` or `--ai-runner opencode`.
  - SliceDiff delegates model selection, model-provider authentication, and provider credentials entirely to the selected agent CLI.
  - SliceDiff never stores Codex, opencode, OpenAI, Anthropic, or local model credentials.

- Runner readiness checks:
  - Codex is ready when the `codex` executable exists and `codex login status` succeeds.
  - opencode is ready when the `opencode` executable exists and `opencode auth list` reports configured authentication.
  - If Codex auth is missing, SliceDiff shows recovery guidance for `codex login`.
  - If opencode auth is missing, SliceDiff shows recovery guidance for `opencode auth login`.

- AI consent:
  - Before the first AI grouping request, SliceDiff shows a consent gate explaining that PR metadata and diff hunks will be sent to the selected agent runner.
  - If the user declines consent, SliceDiff opens raw diff navigation only and does not run Codex or opencode.
  - Consent state is stored locally and can be cleared or changed through configuration.

### Acceptance Criteria

- Opening a PR:
  - The user can launch SliceDiff with a GitHub PR URL, `owner/repo#number`, or a PR number from a local checkout.
  - The app uses `gh` to fetch PR metadata, changed files, and unified diff content.
  - Exactly one PR target is accepted per run.
  - If GitHub authentication is unavailable, the app shows a clear recovery message and does not attempt AI grouping.

- AI semantic slicing:
  - The app sends diff hunks and relevant PR metadata only to the selected Codex or opencode runner.
  - Each generated slice includes a title, concise summary, grouped files, grouped hunks, and grouping rationale.
  - Slices distinguish feature additions, behavior changes, refactors, tests, generated files, and mechanical edits when the diff supports those categories.
  - The user can mark a slice as useful or confusing for later evaluation data.

- Navigation and inspection:
  - The TUI includes a slice list, slice detail panel, and raw hunk inspection panel.
  - The user can move between slices, files, and hunks using keyboard shortcuts.
  - The user can jump from any grouped hunk to its raw diff context.
  - The user can switch between semantic grouping and raw file-by-file diff ordering.
  - If AI is disabled, declined, unavailable, or fails, raw diff navigation remains usable.

- Trust and traceability:
  - Every AI-generated claim about a slice links back to one or more hunks.
  - The app never hides hunks that were not confidently assigned; it places them in an "Unsorted or uncertain" slice.
  - The app labels AI output as generated and supports refresh/regeneration for the same PR.
  - Agents must return navigation and understanding artifacts only, never review comments, approval suggestions, or requested code changes.

- TUI behavior:
  - The app is implemented in Go with Bubbletea, Bubbles, and Lipgloss.
  - The layout uses proportional panel sizing, explicit truncation for text in bordered panels, and correct border height accounting.
  - The app supports terminal resize events without layout corruption.
  - The app remains usable in terminals at least 100 columns wide and 30 rows tall.

### Non-Goals

- SliceDiff will not submit, draft, or suggest GitHub review comments.
- SliceDiff will not approve, request changes, merge, close, or mutate PRs.
- SliceDiff will not automatically split PRs or create stacked diffs.
- SliceDiff will not replace GitHub as the final place where code review decisions are recorded.
- SliceDiff will not attempt full static correctness analysis or vulnerability scanning in the MVP.
- SliceDiff will not target arbitrary multi-PR review sessions in the MVP.
- SliceDiff will not call OpenAI, Anthropic, local model servers, or other model APIs directly in the MVP.
- SliceDiff will not support AI runners other than Codex and opencode in the MVP.

## 3. AI System Requirements: Agent Harness

### Tool Requirements

- `gh` CLI for GitHub PR metadata, changed files, and diff retrieval.
- Codex CLI for one supported AI grouping path.
- opencode CLI for the second supported AI grouping path.
- Local git repository context when available, including current repo remote, branch, and worktree path.
- Optional local cache for PR payloads, agent responses, and user slice feedback.

### Agent Invocation Contract

- SliceDiff invokes AI runners non-interactively as subprocesses.
- Codex path:
  - Use `codex exec`.
  - Provide structured prompt input containing PR metadata, structured hunks, deterministic diff signals, and output rules.
  - Use schema-constrained JSON output where available.

- opencode path:
  - Use `opencode run`.
  - Provide equivalent structured prompt input and output rules.
  - Use JSON or event output support where available.

- Shared runner requirements:
  - The subprocess must receive only the target PR metadata, structured hunks, and deterministic diff signals needed for grouping.
  - The subprocess must not receive repository files beyond the target PR diff unless a future feature explicitly asks for richer context.
  - The subprocess must return semantic slice JSON that SliceDiff can parse deterministically.
  - Malformed output, missing hunk references, timeouts, and non-zero exits are recoverable failures.
  - On recoverable failure, SliceDiff keeps raw diff navigation usable and offers retry or slice regeneration.

### AI Inputs

- PR title, description, author, base branch, head branch, head SHA, and changed-file list.
- Unified diff hunks with file paths, hunk headers, additions, deletions, and rename or move metadata when available.
- Deterministic local signals, including language, module paths, package boundaries, test directories, generated-file markers, lockfile markers, path clusters, and symbol names extracted from hunks.

### AI Outputs

- Ordered list of semantic slices.
- Slice title and 1-3 sentence summary.
- Grouping rationale that names the files, hunks, or symbols tying the slice together.
- Slice category such as feature, behavior change, refactor, tests, generated code, documentation, configuration, or uncertain.
- Hunk references for every grouped claim.
- Optional warnings when the agent could not confidently group part of the diff.

### Evaluation Strategy

- Build a benchmark set of at least 30 large PRs with 20+ changed files across multiple languages.
- Have human reviewers label whether each AI-generated slice is coherent, partially coherent, or confusing.
- Track slice acceptance rate, percentage of hunks assigned to coherent slices, number of hunks left uncertain, and reviewer-reported understanding.
- Compare reviewer comments before and after SliceDiff use, focusing on comments that ask for basic intent clarification.
- Fail evaluation if any generated slice summary cannot be traced back to specific hunks.
- Compare Codex and opencode runner outputs on the same benchmark set without requiring SliceDiff to choose or manage their underlying models.

## 4. Technical Specifications

### Architecture Overview

SliceDiff is a local terminal application built in Go using the Bubbletea Elm architecture:

- Input layer: Parses CLI arguments and resolves the single target GitHub PR.
- GitHub layer: Validates `gh auth status`, calls `gh` to fetch PR metadata, changed files, and diff content, and never mutates GitHub state.
- Diff parser: Converts unified diff text into structured files, hunks, and line-level changes.
- Signal extractor: Adds deterministic context such as file categories, test/source relationships, rename markers, path clusters, generated-file markers, and symbol hints.
- Agent harness: Checks Codex or opencode readiness, builds agent prompts, invokes the selected runner, validates semantic slice JSON, and reports recoverable failures.
- State model: Stores PR metadata, raw hunks, slices, focused panel, selected slice, selected file, selected hunk, selected runner, consent state, loading state, and errors.
- TUI renderer: Displays slice navigation, first-run setup, consent gate, slice details, hunk drill-down, raw diff view, status messages, and help.
- Cache and feedback layer: Stores fetched PR data, generated slices, selected runner, consent state, and local usefulness/confusion feedback.

### CLI & Configuration Requirements

- CLI:
  - Accept exactly one PR target: GitHub PR URL, `owner/repo#number`, or PR number from a local checkout.
  - Support `--ai-runner codex|opencode` to override the saved runner for a single run.
  - Support `--no-ai` to bypass AI consent, runner setup, runner checks, cache lookup for slices, and agent invocation.
  - Support `--regen-slices` to ignore cached slices and rerun the selected agent for the current PR head SHA.

- Configuration:
  - Store config in the user config directory.
  - Store selected AI runner and AI consent state.
  - Do not store GitHub credentials, Codex credentials, opencode credentials, or model-provider API keys.
  - Allow the user to clear selected runner, consent state, and local cache.

- Cache:
  - Cache fetched PR payloads and generated semantic slices locally.
  - Cache keys must include PR owner, repo, PR number, PR head SHA, selected runner, prompt/schema version, and SliceDiff version.
  - Cached slices must be invalidated when the PR head SHA, runner, prompt/schema version, or SliceDiff version changes.

### TUI Layout Requirements

- MVP layout should use a keyboard-first multi-panel interface:
  - Left panel: Semantic slice list.
  - Center panel: Slice explanation, grouped files, and rationale.
  - Right or lower panel: Raw hunk preview and exact diff context.
  - Status bar: Current PR, AI status, selected hunk count, active runner, and keybinding hints.

- Bubbletea implementation requirements:
  - Keep `main.go` minimal and route application behavior through model, update, view, and style files.
  - Separate keyboard and mouse handling into dedicated update paths.
  - Use weight-based panel sizing instead of fixed pixel widths.
  - Account for Lipgloss borders before calculating panel content height.
  - Explicitly truncate strings in bordered panels to avoid accidental wrapping.
  - Match mouse hit detection to the current layout orientation.
  - Support responsive fallback from horizontal panels to vertical stacked panels when terminal width is constrained.

### Integration Points

- GitHub:
  - Primary integration is the installed and authenticated `gh` CLI.
  - The app reads a single target PR and should not mutate GitHub state.

- Codex:
  - SliceDiff checks executable presence and `codex login status`.
  - SliceDiff invokes `codex exec` for semantic slicing.
  - Codex owns model selection and model-provider authentication.

- opencode:
  - SliceDiff checks executable presence and `opencode auth list`.
  - SliceDiff invokes `opencode run` for semantic slicing.
  - opencode owns model selection and model-provider authentication.

- Local filesystem:
  - Configuration lives under the user's config directory.
  - Cache lives under the user's cache directory.
  - No repository files are modified by default.

### Security & Privacy

- SliceDiff must clearly disclose that PR metadata and diff content may be sent to the selected agent runner.
- The app must require first-run AI consent before invoking Codex or opencode.
- The app must support `--no-ai` and a config option to disable AI calls and show raw diff navigation only.
- GitHub authentication must be delegated entirely to `gh`.
- AI authentication and model selection must be delegated entirely to Codex or opencode.
- API keys and model-provider credentials must never be stored, displayed, or logged by SliceDiff.
- Cached PR diffs and AI outputs must be stored locally and may be cleared by the user.
- The app must not send repository files beyond the target PR diff and selected metadata unless a future feature explicitly asks for it.
- The app must not write review comments or modify PR state in GitHub.

### Performance Requirements

- After `gh` returns data, initial parsing should complete within 2 seconds for a PR with 100 changed files and 5,000 changed lines.
- The TUI should remain responsive while AI grouping is in progress by showing loading states and allowing raw diff navigation.
- Cached AI results for the same PR head SHA, runner, prompt/schema version, and SliceDiff version should load within 1 second.
- Navigation between slices and hunks should update within 100ms for benchmark PRs.
- Agent invocation should have a configurable timeout and must fail gracefully without exiting the TUI.

## 5. Risks & Roadmap

### Phased Rollout

- MVP:
  - Open one GitHub PR via `gh`.
  - Validate GitHub authentication through `gh auth status`.
  - Fetch metadata and unified diff.
  - Parse diff into structured files and hunks.
  - Show first-run AI consent and Codex/opencode runner selection.
  - Generate AI semantic slices through the selected runner subprocess.
  - Render Bubbletea TUI with slice list, slice detail, and raw hunk preview.
  - Preserve hunk-level traceability.
  - Provide local cache and basic slice usefulness feedback.

- v1.1:
  - Add better deterministic pre-grouping using symbols, imports, tests, and path clusters.
  - Add regenerated slices with user-selected grouping style.
  - Add export of slice summaries to local markdown without posting to GitHub.
  - Improve support for renamed files, moved code, generated files, and lockfiles.
  - Add explicit config management UI for runner, consent, and cache clearing.

- v2.0:
  - Support richer repository context while preserving explicit privacy controls.
  - Add additional agent runners only if they can satisfy the same subprocess and structured-output contract.
  - Add benchmark tooling for comparing grouping quality across runners.
  - Add optional team-shared slice presets or local review playbooks.

### Technical Risks

- AI grouping may create plausible but incorrect explanations that reduce reviewer trust.
- Large PRs may exceed runner context limits, requiring chunking and hierarchical grouping.
- `gh` output formats or authentication states may vary across user environments.
- Codex and opencode CLI behavior, flags, or output formats may change across installed versions.
- Unified diff parsing can be brittle around binary files, renames, generated files, submodules, and large lockfiles.
- Terminal layout bugs can make the app feel unreliable if borders, wrapping, resizing, or panel focus are mishandled.
- Sending proprietary diff content to Codex or opencode may be unacceptable for some users or organizations, depending on their agent configuration.

### Test Scenarios

- `gh` missing, unauthenticated, inaccessible private PR, and successful authenticated PR load.
- First run with no AI runner configured: picker appears, choice is saved, later run uses saved runner.
- Codex installed and logged in: SliceDiff invokes Codex subprocess and parses valid slice JSON.
- Codex installed but not logged in: SliceDiff shows `codex login` recovery text.
- opencode installed and authenticated: SliceDiff invokes opencode subprocess and parses valid slice JSON.
- opencode missing or unauthenticated: SliceDiff shows install/auth recovery text.
- User declines AI consent: SliceDiff opens raw diff navigation only.
- Agent returns malformed JSON, incomplete hunk references, timeout, or non-zero exit: SliceDiff keeps raw diff usable and shows retry/regenerate action.
- `--no-ai` bypasses runner checks and never sends diff content to Codex or opencode.

### Open Questions

- What minimum terminal size should be officially supported beyond the MVP target of 100 columns by 30 rows?
- Should SliceDiff expose a non-interactive command for benchmarking slice quality in the MVP or defer it to v2.0?
- What exact semantic slice JSON schema should be versioned for the first implementation?
