package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/olekgolus11/SliceDiff/internal/github"
)

type Options struct {
	Runner  RunnerName
	Timeout time.Duration
	WorkDir string
}

func CheckReady(ctx context.Context, runner RunnerName) error {
	switch runner {
	case RunnerCodex:
		if _, err := exec.LookPath("codex"); err != nil {
			return fmt.Errorf("codex CLI not found on PATH")
		}
		return runCheck(ctx, "codex", "login", "status")
	case RunnerOpenCode:
		if _, err := exec.LookPath("opencode"); err != nil {
			return fmt.Errorf("opencode CLI not found on PATH")
		}
		return runCheck(ctx, "opencode", "auth", "list")
	default:
		return fmt.Errorf("unsupported AI runner %q", runner)
	}
}

func Run(ctx context.Context, opts Options, pr github.PullRequest) (*SliceSet, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Minute
	}
	if opts.WorkDir == "" {
		opts.WorkDir, _ = os.Getwd()
	}
	if err := CheckReady(ctx, opts.Runner); err != nil {
		return nil, err
	}

	prompt, err := BuildPrompt(opts.Runner, pr)
	if err != nil {
		return nil, err
	}

	var out []byte
	switch opts.Runner {
	case RunnerCodex:
		out, err = runCodex(ctx, opts, prompt)
	case RunnerOpenCode:
		out, err = runOpenCode(ctx, opts, prompt)
	default:
		err = fmt.Errorf("unsupported AI runner %q", opts.Runner)
	}
	if err != nil {
		return nil, err
	}
	return ParseSliceSet(out, opts.Runner, pr.HeadSHA, pr.Files)
}

func BuildPrompt(runner RunnerName, pr github.PullRequest) ([]byte, error) {
	payload := map[string]any{
		"instructions": []string{
			"You are grouping a single GitHub pull request into semantic review slices for navigation and understanding only.",
			"Do not write review comments, approval language, code-change requests, or suggestions to modify code.",
			"Return JSON only. Do not wrap the JSON in markdown.",
			"Every slice must reference one or more known hunk IDs.",
			"Use the schema_version slicediff.slice.v1.",
		},
		"required_output_shape": map[string]any{
			"schema_version":   SchemaVersion,
			"runner":           string(runner),
			"prompt_version":   PromptVersion,
			"pr_head_sha":      pr.HeadSHA,
			"slices":           []any{},
			"unassigned_hunks": []any{},
			"warnings":         []string{},
		},
		"pull_request": pr,
	}
	return json.MarshalIndent(payload, "", "  ")
}

func runCodex(ctx context.Context, opts Options, prompt []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	schemaPath, cleanup, err := writeSchemaFile()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	outputPath, outputCleanup, err := reserveTempPath("slicediff-codex-output-*.json")
	if err != nil {
		return nil, err
	}
	defer outputCleanup()

	args := []string{"exec", "--skip-git-repo-check", "--ephemeral", "--color", "never", "--output-schema", schemaPath, "--output-last-message", outputPath, "-"}
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = opts.WorkDir
	cmd.Stdin = bytes.NewReader(prompt)
	stdout, err := combinedOutput(cmd, "codex "+strings.Join(args, " "))
	if err != nil {
		return nil, err
	}
	out, err := os.ReadFile(outputPath)
	if err == nil && len(bytes.TrimSpace(out)) > 0 {
		return out, nil
	}
	return stdout, nil
}

func runOpenCode(ctx context.Context, opts Options, prompt []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	promptFile, cleanup, err := writeTempPayload("slicediff-opencode-*.json", prompt)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	message := "Group the attached SliceDiff pull request payload into semantic slices. Return JSON only."
	args := []string{"run", "--format", "json", "--file", promptFile, message}
	cmd := exec.CommandContext(ctx, "opencode", args...)
	cmd.Dir = opts.WorkDir
	return combinedOutput(cmd, "opencode "+strings.Join(args, " "))
}

func runCheck(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s %s failed: %s", name, strings.Join(args, " "), msg)
	}
	return nil
}

func combinedOutput(cmd *exec.Cmd, label string) ([]byte, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s failed: %s", label, msg)
	}
	return out, nil
}

func writeSchemaFile() (string, func(), error) {
	schema := []byte(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["schema_version", "runner", "prompt_version", "pr_head_sha", "slices", "unassigned_hunks", "warnings"],
  "properties": {
    "schema_version": {"type": "string", "const": "slicediff.slice.v1"},
    "runner": {"type": "string"},
    "prompt_version": {"type": "string"},
    "pr_head_sha": {"type": "string"},
    "warnings": {"type": "array", "items": {"type": "string"}},
    "unassigned_hunks": {
      "type": "array",
      "items": {"$ref": "#/$defs/hunk_ref"}
    },
    "slices": {
      "type": "array",
      "items": {"$ref": "#/$defs/slice"}
    }
  },
  "$defs": {
    "hunk_ref": {
      "type": "object",
      "additionalProperties": false,
      "required": ["hunk_id", "file_path", "header"],
      "properties": {
        "hunk_id": {"type": "string"},
        "file_path": {"type": "string"},
        "header": {"type": "string"}
      }
    },
    "slice": {
      "type": "object",
      "additionalProperties": false,
      "required": ["id", "title", "summary", "category", "confidence", "rationale", "hunk_refs"],
      "properties": {
        "id": {"type": "string"},
        "title": {"type": "string"},
        "summary": {"type": "string"},
        "category": {"type": "string"},
        "confidence": {"type": "string"},
        "rationale": {"type": "string"},
        "hunk_refs": {"type": "array", "items": {"$ref": "#/$defs/hunk_ref"}}
      }
    }
  }
}`)
	return writeTempPayload("slicediff-schema-*.json", schema)
}

func writeTempPayload(pattern string, payload []byte) (string, func(), error) {
	dir := os.TempDir()
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if _, err := file.Write(payload); err != nil {
		file.Close()
		os.Remove(path)
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", func() {}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return abs, func() { _ = os.Remove(abs) }, nil
}

func reserveTempPath(pattern string) (string, func(), error) {
	file, err := os.CreateTemp(os.TempDir(), pattern)
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", func() {}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return abs, func() { _ = os.Remove(abs) }, nil
}
