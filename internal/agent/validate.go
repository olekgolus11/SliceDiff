package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/olekgolus11/SliceDiff/internal/diff"
)

func ParseSliceSet(raw []byte, runner RunnerName, headSHA string, files []diff.DiffFile) (*SliceSet, error) {
	raw = extractJSONObject(raw)
	var set SliceSet
	if err := json.Unmarshal(raw, &set); err != nil {
		return nil, fmt.Errorf("agent output is not valid slice JSON: %w", err)
	}
	if err := ValidateSliceSet(&set, runner, headSHA, files); err != nil {
		return nil, err
	}
	return &set, nil
}

func ValidateSliceSet(set *SliceSet, runner RunnerName, headSHA string, files []diff.DiffFile) error {
	if set.SchemaVersion != SchemaVersion {
		return fmt.Errorf("agent output schema_version must be %q", SchemaVersion)
	}
	if set.Runner == "" {
		set.Runner = string(runner)
	}
	if set.Runner != string(runner) {
		return fmt.Errorf("agent output runner %q does not match selected runner %q", set.Runner, runner)
	}
	if set.PromptVersion == "" {
		set.PromptVersion = PromptVersion
	}
	if set.PRHeadSHA == "" {
		set.PRHeadSHA = headSHA
	}
	if set.PRHeadSHA != headSHA {
		return fmt.Errorf("agent output head SHA does not match PR head SHA")
	}

	hunks := map[string]diff.DiffHunk{}
	for _, file := range files {
		for _, hunk := range file.Hunks {
			hunks[hunk.ID] = hunk
		}
	}

	seenSlices := map[string]bool{}
	for _, slice := range set.Slices {
		if strings.TrimSpace(slice.ID) == "" {
			return fmt.Errorf("agent output contains a slice without an id")
		}
		if seenSlices[slice.ID] {
			return fmt.Errorf("agent output contains duplicate slice id %q", slice.ID)
		}
		seenSlices[slice.ID] = true
		if strings.TrimSpace(slice.Title) == "" {
			return fmt.Errorf("agent output slice %q is missing a title", slice.ID)
		}
		if len(slice.HunkRefs) == 0 {
			return fmt.Errorf("agent output slice %q has no hunk references", slice.ID)
		}
		if len(slice.ReadingSteps) != len(slice.HunkRefs) {
			return fmt.Errorf("agent output slice %q must include one reading step per hunk reference", slice.ID)
		}
		seenStepHunks := map[string]bool{}
		for i, ref := range slice.HunkRefs {
			if _, ok := hunks[ref.HunkID]; !ok {
				return fmt.Errorf("agent output references unknown hunk %q", ref.HunkID)
			}
			step := slice.ReadingSteps[i]
			if strings.TrimSpace(step.Body) == "" {
				return fmt.Errorf("agent output slice %q reading step %d is missing body", slice.ID, i+1)
			}
			if seenStepHunks[step.HunkRef.HunkID] {
				return fmt.Errorf("agent output slice %q contains duplicate reading step hunk %q", slice.ID, step.HunkRef.HunkID)
			}
			seenStepHunks[step.HunkRef.HunkID] = true
			if step.HunkRef.HunkID != ref.HunkID {
				return fmt.Errorf("agent output slice %q reading step %d must reference hunk %q", slice.ID, i+1, ref.HunkID)
			}
			if _, ok := hunks[step.HunkRef.HunkID]; !ok {
				return fmt.Errorf("agent output references unknown reading step hunk %q", step.HunkRef.HunkID)
			}
		}
	}
	for _, ref := range set.UnassignedHunks {
		if _, ok := hunks[ref.HunkID]; !ok {
			return fmt.Errorf("agent output references unknown unassigned hunk %q", ref.HunkID)
		}
	}
	return nil
}

func extractJSONObject(raw []byte) []byte {
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			text = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return []byte(text[start : end+1])
	}
	return []byte(text)
}
