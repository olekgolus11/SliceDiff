package tui

import (
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		kind ErrorKind
	}{
		{"gh auth", errors.New("GitHub CLI is not authenticated or cannot access GitHub; run gh auth login"), ErrorGitHubAuth},
		{"missing codex", errors.New("codex CLI not found on PATH"), ErrorMissingTool},
		{"runner auth", errors.New("codex login status failed: not logged in"), ErrorRunnerAuth},
		{"invalid schema", errors.New("invalid_json_schema: schema must have a type key"), ErrorInvalidSchema},
		{"malformed json", errors.New("agent output is not valid slice JSON: invalid character 'x'"), ErrorMalformedJSON},
		{"timeout", errors.New("context deadline exceeded"), ErrorTimeout},
		{"unknown hunk", errors.New("agent output references unknown hunk h999"), ErrorUnknownHunk},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyError(tc.err)
			if got == nil {
				t.Fatal("expected classified error")
			}
			if got.Kind != tc.kind {
				t.Fatalf("expected kind %q, got %q", tc.kind, got.Kind)
			}
			if got.Summary == "" || got.Recovery == "" || got.Detail == "" {
				t.Fatalf("classification missing text: %+v", got)
			}
		})
	}
}
