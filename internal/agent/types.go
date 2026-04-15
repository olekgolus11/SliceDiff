package agent

import "github.com/olekgolus11/SliceDiff/internal/diff"

const (
	SchemaVersion = "slicediff.slice.v3"
	PromptVersion = "prompt.v5"
)

type RunnerName string

const (
	RunnerCodex    RunnerName = "codex"
	RunnerOpenCode RunnerName = "opencode"
)

func IsSupportedRunner(name string) bool {
	return RunnerName(name) == RunnerCodex || RunnerName(name) == RunnerOpenCode
}

type SliceSet struct {
	SchemaVersion   string    `json:"schema_version" yaml:"schema_version"`
	Runner          string    `json:"runner" yaml:"runner"`
	PromptVersion   string    `json:"prompt_version" yaml:"prompt_version"`
	PRHeadSHA       string    `json:"pr_head_sha" yaml:"pr_head_sha"`
	Slices          []Slice   `json:"slices" yaml:"slices"`
	UnassignedHunks []HunkRef `json:"unassigned_hunks" yaml:"unassigned_hunks"`
	Warnings        []string  `json:"warnings" yaml:"warnings"`
}

type Slice struct {
	ID           string        `json:"id" yaml:"id"`
	Title        string        `json:"title" yaml:"title"`
	Summary      string        `json:"summary" yaml:"summary"`
	Category     string        `json:"category" yaml:"category"`
	Rationale    string        `json:"rationale" yaml:"rationale"`
	HunkRefs     []HunkRef     `json:"hunk_refs" yaml:"hunk_refs"`
	ReadingSteps []ReadingStep `json:"reading_steps" yaml:"reading_steps"`
}

type ReadingStep struct {
	HunkRef HunkRef `json:"hunk_ref" yaml:"hunk_ref"`
	Body    string  `json:"body" yaml:"body"`
}

type HunkRef struct {
	HunkID   string `json:"hunk_id" yaml:"hunk_id"`
	FilePath string `json:"file_path" yaml:"file_path"`
	Header   string `json:"header" yaml:"header"`
}

func RefForHunk(h diff.DiffHunk) HunkRef {
	return HunkRef{
		HunkID:   h.ID,
		FilePath: h.FilePath,
		Header:   h.Header,
	}
}
