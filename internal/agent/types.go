package agent

import "github.com/olekgolus11/SliceDiff/internal/diff"

const (
	SchemaVersion = "slicediff.slice.v1"
	PromptVersion = "prompt.v1"
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
	ID         string    `json:"id" yaml:"id"`
	Title      string    `json:"title" yaml:"title"`
	Summary    string    `json:"summary" yaml:"summary"`
	Category   string    `json:"category" yaml:"category"`
	Confidence string    `json:"confidence" yaml:"confidence"`
	Rationale  string    `json:"rationale" yaml:"rationale"`
	HunkRefs   []HunkRef `json:"hunk_refs" yaml:"hunk_refs"`
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
