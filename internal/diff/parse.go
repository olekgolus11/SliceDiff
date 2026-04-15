package diff

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var hunkHeaderRE = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@`)

func ParseUnified(raw string) ([]DiffFile, error) {
	scanner := bufio.NewScanner(strings.NewReader(raw))
	scanner.Buffer(make([]byte, 1024), 10*1024*1024)

	var files []DiffFile
	var current *DiffFile
	var currentHunk *DiffHunk
	var oldLine int
	var newLine int
	hunkSeq := 0

	finishHunk := func() {
		if current != nil && currentHunk != nil {
			currentHunk.Signal, currentHunk.Reason = ClassifyHunk(current.Path, *currentHunk)
			current.Hunks = append(current.Hunks, *currentHunk)
			currentHunk = nil
		}
	}
	finishFile := func() {
		if current != nil {
			finishHunk()
			current.IsGenerated = IsGeneratedPath(current.Path)
			files = append(files, *current)
			current = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git ") {
			finishFile()
			oldPath, newPath := parseDiffGitLine(line)
			current = &DiffFile{
				Path:    newPath,
				OldPath: oldPath,
				Status:  "modified",
			}
			continue
		}
		if current == nil {
			continue
		}

		switch {
		case strings.HasPrefix(line, "new file mode "):
			current.Status = "added"
		case strings.HasPrefix(line, "deleted file mode "):
			current.Status = "deleted"
		case strings.HasPrefix(line, "rename from "):
			current.Status = "renamed"
			current.OldPath = strings.TrimPrefix(line, "rename from ")
		case strings.HasPrefix(line, "rename to "):
			current.Status = "renamed"
			current.Path = strings.TrimPrefix(line, "rename to ")
		case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
			current.IsBinary = true
		case strings.HasPrefix(line, "--- "):
			old := trimDiffPath(strings.TrimPrefix(line, "--- "))
			if old == "/dev/null" {
				current.Status = "added"
			} else if old != "" {
				current.OldPath = old
			}
		case strings.HasPrefix(line, "+++ "):
			newPath := trimDiffPath(strings.TrimPrefix(line, "+++ "))
			if newPath == "/dev/null" {
				current.Status = "deleted"
			} else if newPath != "" {
				current.Path = newPath
			}
		case strings.HasPrefix(line, "@@ "):
			finishHunk()
			h, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			hunkSeq++
			h.ID = fmt.Sprintf("h%d", hunkSeq)
			h.FilePath = current.Path
			currentHunk = &h
			oldLine = h.OldStart
			newLine = h.NewStart
		case currentHunk != nil:
			if strings.HasPrefix(line, `\ No newline at end of file`) {
				continue
			}
			if line == "" {
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: LineContext, OldNumber: oldLine, NewNumber: newLine})
				oldLine++
				newLine++
				continue
			}
			prefix := line[0]
			content := ""
			if len(line) > 1 {
				content = line[1:]
			}
			switch prefix {
			case '+':
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: LineAdded, NewNumber: newLine, Content: content})
				newLine++
			case '-':
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: LineDeleted, OldNumber: oldLine, Content: content})
				oldLine++
			default:
				currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: LineContext, OldNumber: oldLine, NewNumber: newLine, Content: content})
				oldLine++
				newLine++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	finishFile()
	return files, nil
}

func parseDiffGitLine(line string) (string, string) {
	rest := strings.TrimPrefix(line, "diff --git ")
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return trimDiffPath(parts[0]), trimDiffPath(parts[1])
}

func parseHunkHeader(header string) (DiffHunk, error) {
	match := hunkHeaderRE.FindStringSubmatch(header)
	if match == nil {
		return DiffHunk{}, fmt.Errorf("invalid hunk header %q", header)
	}
	oldStart, _ := strconv.Atoi(match[1])
	oldLines := parseCount(match[2])
	newStart, _ := strconv.Atoi(match[3])
	newLines := parseCount(match[4])
	return DiffHunk{
		Header:   header,
		OldStart: oldStart,
		OldLines: oldLines,
		NewStart: newStart,
		NewLines: newLines,
	}, nil
}

func parseCount(raw string) int {
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 1
	}
	return n
}

func trimDiffPath(path string) string {
	path = strings.Trim(path, `"`)
	if strings.HasPrefix(path, "a/") || strings.HasPrefix(path, "b/") {
		return path[2:]
	}
	return path
}

func IsGeneratedPath(path string) bool {
	lower := strings.ToLower(path)
	generatedMarkers := []string{
		"generated",
		".gen.",
		"_gen.",
		"vendor/",
		"dist/",
		"build/",
	}
	for _, marker := range generatedMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	for _, lockfile := range lockfileNames() {
		if strings.HasSuffix(lower, lockfile) {
			return true
		}
	}
	return false
}
