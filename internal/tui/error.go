package tui

import (
	"strings"
)

type ErrorKind string

const (
	ErrorNone          ErrorKind = ""
	ErrorGitHubAuth    ErrorKind = "github-auth"
	ErrorMissingTool   ErrorKind = "missing-tool"
	ErrorRunnerAuth    ErrorKind = "runner-auth"
	ErrorInvalidSchema ErrorKind = "invalid-schema"
	ErrorMalformedJSON ErrorKind = "malformed-json"
	ErrorTimeout       ErrorKind = "timeout"
	ErrorUnknownHunk   ErrorKind = "unknown-hunk"
	ErrorAgent         ErrorKind = "agent"
	ErrorUnknown       ErrorKind = "unknown"
)

type AppError struct {
	Kind     ErrorKind
	Summary  string
	Detail   string
	Recovery string
}

func classifyError(err error) *AppError {
	if err == nil {
		return nil
	}
	detail := err.Error()
	lower := strings.ToLower(detail)
	appErr := &AppError{
		Kind:     ErrorUnknown,
		Summary:  "Something went wrong.",
		Detail:   detail,
		Recovery: "Try again, or run with --no-ai to inspect the raw diff.",
	}

	switch {
	case strings.Contains(lower, "gh cli not found"):
		appErr.Kind = ErrorMissingTool
		appErr.Summary = "GitHub CLI is not installed."
		appErr.Recovery = "Install GitHub CLI, then run gh auth login."
	case strings.Contains(lower, "gh auth") || strings.Contains(lower, "github cli is not authenticated"):
		appErr.Kind = ErrorGitHubAuth
		appErr.Summary = "GitHub authentication is not ready."
		appErr.Recovery = "Run gh auth login, then start SliceDiff again."
	case strings.Contains(lower, "codex cli not found"):
		appErr.Kind = ErrorMissingTool
		appErr.Summary = "Codex CLI is not installed."
		appErr.Recovery = "Install Codex or run with --ai-runner opencode or --no-ai."
	case strings.Contains(lower, "opencode cli not found"):
		appErr.Kind = ErrorMissingTool
		appErr.Summary = "opencode CLI is not installed."
		appErr.Recovery = "Install opencode or run with --ai-runner codex or --no-ai."
	case strings.Contains(lower, "codex login") || strings.Contains(lower, "opencode auth"):
		appErr.Kind = ErrorRunnerAuth
		appErr.Summary = "The selected AI runner is not authenticated."
		appErr.Recovery = "Run codex login or opencode auth login, then retry with r or --regen-slices."
	case strings.Contains(lower, "invalid_json_schema") || strings.Contains(lower, "schema must"):
		appErr.Kind = ErrorInvalidSchema
		appErr.Summary = "The AI output schema was rejected."
		appErr.Recovery = "Update SliceDiff, then retry with --regen-slices. Use --no-ai for raw review meanwhile."
	case strings.Contains(lower, "not valid slice json") || strings.Contains(lower, "invalid character"):
		appErr.Kind = ErrorMalformedJSON
		appErr.Summary = "The AI runner returned malformed JSON."
		appErr.Recovery = "Press r to retry, or run with --no-ai to inspect the raw diff."
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(lower, "timed out") || strings.Contains(lower, "timeout"):
		appErr.Kind = ErrorTimeout
		appErr.Summary = "The AI runner timed out."
		appErr.Recovery = "Press r to retry, or use --no-ai for raw diff navigation."
	case strings.Contains(lower, "unknown hunk"):
		appErr.Kind = ErrorUnknownHunk
		appErr.Summary = "The AI runner referenced a hunk that does not exist."
		appErr.Recovery = "Retry with --regen-slices. SliceDiff will keep raw diff navigation available."
	case strings.Contains(lower, "codex exec") || strings.Contains(lower, "opencode run"):
		appErr.Kind = ErrorAgent
		appErr.Summary = "AI grouping failed."
		appErr.Recovery = "Check the details below, then press r to retry or run with --no-ai."
	}

	return appErr
}
