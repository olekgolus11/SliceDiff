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
		"schema_version": "slicediff.slice.v3",
		"runner": "codex",
		"prompt_version": "prompt.v2",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"}],
			"reading_steps": [{
				"hunk_ref": {"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"},
				"body": "The author updates main so the behavior changes at the entrypoint."
			}]
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
		"schema_version": "slicediff.slice.v3",
		"runner": "codex",
		"prompt_version": "prompt.v2",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h999", "file_path": "main.go", "header": "@@ -1 +1 @@"}],
			"reading_steps": [{
				"hunk_ref": {"hunk_id": "h999", "file_path": "main.go", "header": "@@ -1 +1 @@"},
				"body": "The author updates main so the behavior changes at the entrypoint."
			}]
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	if _, err := ParseSliceSet(raw, RunnerCodex, "abc", files); err == nil {
		t.Fatal("expected unknown hunk validation error")
	}
}

func TestParseSliceSetRejectsMissingReadingStep(t *testing.T) {
	files := []diff.DiffFile{{
		Path: "main.go",
		Hunks: []diff.DiffHunk{{
			ID:       "h1",
			FilePath: "main.go",
			Header:   "@@ -1 +1 @@",
		}},
	}}
	raw := []byte(`{
		"schema_version": "slicediff.slice.v3",
		"runner": "codex",
		"prompt_version": "prompt.v2",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"}],
			"reading_steps": []
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	if _, err := ParseSliceSet(raw, RunnerCodex, "abc", files); err == nil {
		t.Fatal("expected missing reading step validation error")
	}
}

func TestParseSliceSetRejectsDuplicateReadingStep(t *testing.T) {
	files := []diff.DiffFile{{
		Path: "main.go",
		Hunks: []diff.DiffHunk{
			{ID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"},
			{ID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"},
		},
	}}
	raw := []byte(`{
		"schema_version": "slicediff.slice.v3",
		"runner": "codex",
		"prompt_version": "prompt.v2",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [
				{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"},
				{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"}
			],
			"reading_steps": [{
				"hunk_ref": {"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"},
				"body": "The author updates main first."
			}, {
				"hunk_ref": {"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"},
				"body": "The author updates main again."
			}]
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	if _, err := ParseSliceSet(raw, RunnerCodex, "abc", files); err == nil {
		t.Fatal("expected duplicate reading step validation error")
	}
}

func TestParseSliceSetRejectsOutOfSliceReadingStep(t *testing.T) {
	files := []diff.DiffFile{{
		Path: "main.go",
		Hunks: []diff.DiffHunk{
			{ID: "h1", FilePath: "main.go", Header: "@@ -1 +1 @@"},
			{ID: "h2", FilePath: "main.go", Header: "@@ -2 +2 @@"},
		},
	}}
	raw := []byte(`{
		"schema_version": "slicediff.slice.v3",
		"runner": "codex",
		"prompt_version": "prompt.v2",
		"pr_head_sha": "abc",
		"slices": [{
			"id": "s1",
			"title": "Update main",
			"summary": "Updates main.",
			"category": "behavior",
			"rationale": "The hunk changes main.go.",
			"hunk_refs": [{"hunk_id": "h1", "file_path": "main.go", "header": "@@ -1 +1 @@"}],
			"reading_steps": [{
				"hunk_ref": {"hunk_id": "h2", "file_path": "main.go", "header": "@@ -2 +2 @@"},
				"body": "The author updates a different hunk."
			}]
		}],
		"unassigned_hunks": [],
		"warnings": []
	}`)
	if _, err := ParseSliceSet(raw, RunnerCodex, "abc", files); err == nil {
		t.Fatal("expected out-of-slice reading step validation error")
	}
}
