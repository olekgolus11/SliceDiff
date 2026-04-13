package agent

import (
	"testing"

	"github.com/olekgolus11/SliceDiff/internal/diff"
)

func TestParseSliceSetValidatesKnownHunks(t *testing.T) {
	files := []diff.DiffFile{{
		Path: "main.go",
		Hunks: []diff.DiffHunk{{
			ID:       "h1",
			FilePath: "main.go",
			Header:   "@@ -1 +1 @@",
		}},
	}}
	raw := []byte(`{
		"schema_version": "slicediff.slice.v1",
		"runner": "codex",
		"prompt_version": "prompt.v1",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"confidence": "high",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"}]
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	set, err := ParseSliceSet(raw, RunnerCodex, "abc", files)
	if err != nil {
		t.Fatalf("ParseSliceSet returned error: %v", err)
	}
	if len(set.Slices) != 1 {
		t.Fatalf("expected one slice, got %+v", set)
	}
}

func TestParseSliceSetRejectsUnknownHunk(t *testing.T) {
	files := []diff.DiffFile{{
		Path: "main.go",
		Hunks: []diff.DiffHunk{{
			ID:       "h1",
			FilePath: "main.go",
			Header:   "@@ -1 +1 @@",
		}},
	}}
	raw := []byte(`{
		"schema_version": "slicediff.slice.v1",
		"runner": "codex",
		"prompt_version": "prompt.v1",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"confidence": "high",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h999", "file_path": "main.go", "header": "@@ -1 +1 @@"}]
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	if _, err := ParseSliceSet(raw, RunnerCodex, "abc", files); err == nil {
		t.Fatal("expected unknown hunk validation error")
	}
}
