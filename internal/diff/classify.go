package diff

import (
	"path/filepath"
	"strings"
	"unicode"
)

func ClassifyHunk(path string, hunk DiffHunk) (HunkSignal, string) {
	if ok, reason := auditPathReason(path); ok {
		return HunkSignalAudit, reason
	}
	if isWhitespaceOnlyChange(hunk) {
		return HunkSignalQuiet, "whitespace"
	}
	if isImportOnlyChange(path, hunk) {
		return HunkSignalQuiet, "imports"
	}
	if isFormatOnlyChange(hunk) {
		return HunkSignalQuiet, "format"
	}
	return HunkSignalFocus, ""
}

func auditPathReason(path string) (bool, string) {
	lower := strings.ToLower(path)
	if IsLockfilePath(path) {
		return true, "lockfile"
	}
	if strings.Contains(lower, "vendor/") {
		return true, "vendor"
	}
	if IsGeneratedPath(path) {
		return true, "generated"
	}
	return false, ""
}

func IsLockfilePath(path string) bool {
	lower := strings.ToLower(path)
	for _, lockfile := range lockfileNames() {
		if strings.HasSuffix(lower, lockfile) {
			return true
		}
	}
	return false
}

func isWhitespaceOnlyChange(hunk DiffHunk) bool {
	deleted, added := changedContent(hunk)
	if len(deleted) == 0 || len(added) == 0 || len(deleted) != len(added) {
		return false
	}
	for i := range deleted {
		if removeWhitespace(deleted[i]) != removeWhitespace(added[i]) {
			return false
		}
	}
	return true
}

func isFormatOnlyChange(hunk DiffHunk) bool {
	deleted, added := changedContent(hunk)
	if len(deleted) == 0 || len(added) == 0 {
		return false
	}
	return normalizeJoined(deleted) == normalizeJoined(added)
}

func isImportOnlyChange(path string, hunk DiffHunk) bool {
	changed := changedLines(hunk)
	if len(changed) == 0 {
		return false
	}
	for _, line := range changed {
		if isImportLikeLine(path, line) {
			continue
		}
		return false
	}
	return true
}

func isImportLikeLine(path, line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "import" || trimmed == "import(" || trimmed == "import (" || trimmed == "(" || trimmed == ")" || trimmed == "{" || trimmed == "}" {
		return true
	}
	if filepath.Ext(path) == ".go" && isGoImportSpec(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "import ") || strings.HasPrefix(trimmed, "import\t") {
		return true
	}
	if strings.HasPrefix(trimmed, "from ") && strings.Contains(trimmed, " import ") {
		return true
	}
	if strings.HasPrefix(trimmed, "use ") {
		return true
	}
	if strings.HasPrefix(trimmed, "#include ") {
		return true
	}
	if strings.HasPrefix(trimmed, "using ") && strings.Contains(trimmed, ".") {
		return true
	}
	if strings.HasPrefix(trimmed, "require(") || strings.HasPrefix(trimmed, "const ") && strings.Contains(trimmed, " require(") {
		return true
	}
	return false
}

func isGoImportSpec(line string) bool {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	if strings.HasPrefix(fields[len(fields)-1], `"`) || strings.HasPrefix(fields[len(fields)-1], "`") {
		return true
	}
	return false
}

func changedContent(hunk DiffHunk) ([]string, []string) {
	var deleted []string
	var added []string
	for _, line := range hunk.Lines {
		switch line.Type {
		case LineDeleted:
			deleted = append(deleted, line.Content)
		case LineAdded:
			added = append(added, line.Content)
		}
	}
	return deleted, added
}

func changedLines(hunk DiffHunk) []string {
	var lines []string
	for _, line := range hunk.Lines {
		if line.Type == LineDeleted || line.Type == LineAdded {
			lines = append(lines, line.Content)
		}
	}
	return lines
}

func normalizeJoined(lines []string) string {
	var b strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		b.WriteString(removeWhitespace(trimmed))
	}
	return b.String()
}

func removeWhitespace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func lockfileNames() []string {
	return []string{
		"go.sum",
		"package-lock.json",
		"pnpm-lock.yaml",
		"yarn.lock",
		"bun.lockb",
		"cargo.lock",
		"composer.lock",
		"poetry.lock",
	}
}
