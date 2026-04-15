package diff

type LineType string

const (
	LineContext LineType = "context"
	LineAdded   LineType = "added"
	LineDeleted LineType = "deleted"
)

type HunkSignal string

const (
	HunkSignalFocus HunkSignal = "focus"
	HunkSignalQuiet HunkSignal = "quiet"
	HunkSignalAudit HunkSignal = "audit"
)

type DiffLine struct {
	Type      LineType `json:"type" yaml:"type"`
	OldNumber int      `json:"old_number,omitempty" yaml:"old_number,omitempty"`
	NewNumber int      `json:"new_number,omitempty" yaml:"new_number,omitempty"`
	Content   string   `json:"content" yaml:"content"`
}

type DiffHunk struct {
	ID       string     `json:"id" yaml:"id"`
	FilePath string     `json:"file_path" yaml:"file_path"`
	Header   string     `json:"header" yaml:"header"`
	OldStart int        `json:"old_start" yaml:"old_start"`
	OldLines int        `json:"old_lines" yaml:"old_lines"`
	NewStart int        `json:"new_start" yaml:"new_start"`
	NewLines int        `json:"new_lines" yaml:"new_lines"`
	Lines    []DiffLine `json:"lines" yaml:"lines"`
	Signal   HunkSignal `json:"signal" yaml:"signal"`
	Reason   string     `json:"reason,omitempty" yaml:"reason,omitempty"`
}

type DiffFile struct {
	Path        string     `json:"path" yaml:"path"`
	OldPath     string     `json:"old_path,omitempty" yaml:"old_path,omitempty"`
	Status      string     `json:"status" yaml:"status"`
	IsBinary    bool       `json:"is_binary" yaml:"is_binary"`
	IsGenerated bool       `json:"is_generated" yaml:"is_generated"`
	Hunks       []DiffHunk `json:"hunks" yaml:"hunks"`
}
