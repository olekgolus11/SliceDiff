package agent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/olekgolus11/SliceDiff/internal/diff"
	"github.com/olekgolus11/SliceDiff/internal/github"
)

func TestWriteSchemaFileIncludesTypesForConstFields(t *testing.T) {
	path, cleanup, err := writeSchemaFile()
	if err != nil {
		t.Fatalf("writeSchemaFile returned error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read schema file: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing or invalid: %#v", schema["properties"])
	}
	schemaVersion, ok := properties["schema_version"].(map[string]any)
	if !ok {
		t.Fatalf("schema_version property missing or invalid: %#v", properties["schema_version"])
	}
	if schemaVersion["type"] != "string" {
		t.Fatalf("schema_version must declare type string, got %#v", schemaVersion["type"])
	}
	if schemaVersion["const"] != SchemaVersion {
		t.Fatalf("schema_version const mismatch: %#v", schemaVersion["const"])
	}

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema defs missing or invalid: %#v", schema["$defs"])
	}
	sliceDef, ok := defs["slice"].(map[string]any)
	if !ok {
		t.Fatalf("slice definition missing or invalid: %#v", defs["slice"])
	}
	required, ok := sliceDef["required"].([]any)
	if !ok {
		t.Fatalf("slice required fields missing or invalid: %#v", sliceDef["required"])
	}
	if !containsString(required, "reading_steps") {
		t.Fatalf("slice schema must require reading_steps, got %#v", required)
	}
	if containsString(required, "confidence") {
		t.Fatalf("slice schema must not require confidence, got %#v", required)
	}
	propertiesDef, ok := sliceDef["properties"].(map[string]any)
	if !ok {
		t.Fatalf("slice properties missing or invalid: %#v", sliceDef["properties"])
	}
	if _, ok := propertiesDef["confidence"]; ok {
		t.Fatalf("slice schema must not expose confidence, got %#v", propertiesDef)
	}
	stepDef, ok := defs["reading_step"].(map[string]any)
	if !ok {
		t.Fatalf("reading_step definition missing or invalid: %#v", defs["reading_step"])
	}
	stepRequired, ok := stepDef["required"].([]any)
	if !ok {
		t.Fatalf("reading_step required fields missing or invalid: %#v", stepDef["required"])
	}
	if !containsString(stepRequired, "body") {
		t.Fatalf("reading_step schema must require body, got %#v", stepRequired)
	}
}

func TestCodexExecArgsPinModel(t *testing.T) {
	args := codexExecArgs("schema.json", "out.json")
	if !containsString(stringSliceToAny(args), "--model") {
		t.Fatalf("expected codex args to include --model, got %#v", args)
	}
	for i, arg := range args {
		if arg == "--model" {
			if i+1 >= len(args) || args[i+1] != "gpt-5.4-mini" {
				t.Fatalf("expected codex model gpt-5.4-mini after --model, got %#v", args)
			}
			return
		}
	}
	t.Fatalf("expected codex args to include model flag, got %#v", args)
}

func stringSliceToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBuildPromptIncludesHunkSignalMetadata(t *testing.T) {
	pr := github.PullRequest{
		HeadSHA: "abc123",
		Files: []diff.DiffFile{{
			Path: "main.go",
			Hunks: []diff.DiffHunk{{
				ID:       "h1",
				FilePath: "main.go",
				Header:   "@@ -1 +1 @@",
				Signal:   diff.HunkSignalQuiet,
				Reason:   "imports",
			}},
		}},
	}
	raw, err := BuildPrompt(RunnerCodex, pr)
	if err != nil {
		t.Fatalf("BuildPrompt returned error: %v", err)
	}
	var payload struct {
		Instructions []string           `json:"instructions"`
		PullRequest  github.PullRequest `json:"pull_request"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("prompt is not valid JSON: %v", err)
	}
	if PromptVersion != "prompt.v5" {
		t.Fatalf("expected prompt version bump to prompt.v5, got %q", PromptVersion)
	}
	if got := payload.PullRequest.Files[0].Hunks[0].Signal; got != diff.HunkSignalQuiet {
		t.Fatalf("expected prompt hunk signal metadata, got %q", got)
	}
	if got := payload.PullRequest.Files[0].Hunks[0].Reason; got != "imports" {
		t.Fatalf("expected prompt hunk reason metadata, got %q", got)
	}
	foundQuietInstruction := false
	foundAuditInstruction := false
	for _, instruction := range payload.Instructions {
		if strings.Contains(instruction, "signal=quiet") {
			foundQuietInstruction = true
		}
		if strings.Contains(instruction, "signal=audit") {
			foundAuditInstruction = true
		}
	}
	if !foundQuietInstruction {
		t.Fatalf("expected prompt to instruct quiet hunk handling, got %#v", payload.Instructions)
	}
	if !foundAuditInstruction {
		t.Fatalf("expected prompt to instruct audit hunk handling, got %#v", payload.Instructions)
	}
}
