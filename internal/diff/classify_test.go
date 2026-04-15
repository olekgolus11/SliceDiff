package diff

import "testing"

func TestClassifyWhitespaceOnlyHunkAsQuiet(t *testing.T) {
	hunk := DiffHunk{Lines: []DiffLine{
		{Type: LineDeleted, Content: "if err!=nil{return err}"},
		{Type: LineAdded, Content: "if err != nil { return err }"},
	}}
	signal, reason := ClassifyHunk("main.go", hunk)
	if signal != HunkSignalQuiet || reason != "whitespace" {
		t.Fatalf("expected whitespace quiet hunk, got %s/%q", signal, reason)
	}
}

func TestClassifyGoImportOnlyHunkAsQuiet(t *testing.T) {
	hunk := DiffHunk{Lines: []DiffLine{
		{Type: LineDeleted, Content: "\t\"fmt\""},
		{Type: LineAdded, Content: "\t\"strings\""},
		{Type: LineAdded, Content: "\t\"fmt\""},
	}}
	signal, reason := ClassifyHunk("main.go", hunk)
	if signal != HunkSignalQuiet || reason != "imports" {
		t.Fatalf("expected import quiet hunk, got %s/%q", signal, reason)
	}
}

func TestClassifyCommonImportOnlyHunksAsQuiet(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{name: "typescript", path: "app.ts", old: "import { a } from './a';", new: "import { b } from './b';"},
		{name: "python", path: "app.py", old: "from old import thing", new: "from new import thing"},
		{name: "kotlin", path: "App.kt", old: "import old.Thing", new: "import new.Thing"},
		{name: "rust", path: "lib.rs", old: "use old::Thing;", new: "use new::Thing;"},
		{name: "cpp", path: "app.cc", old: "#include <old.h>", new: "#include <new.h>"},
	}
	for _, tt := range tests {
		hunk := DiffHunk{Lines: []DiffLine{
			{Type: LineDeleted, Content: tt.old},
			{Type: LineAdded, Content: tt.new},
		}}
		signal, reason := ClassifyHunk(tt.path, hunk)
		if signal != HunkSignalQuiet || reason != "imports" {
			t.Fatalf("%s: expected import quiet hunk, got %s/%q", tt.name, signal, reason)
		}
	}
}

func TestClassifyNormalLogicHunkAsFocus(t *testing.T) {
	hunk := DiffHunk{Lines: []DiffLine{
		{Type: LineDeleted, Content: "return oldValue"},
		{Type: LineAdded, Content: "return newValue"},
	}}
	signal, reason := ClassifyHunk("internal/app.go", hunk)
	if signal != HunkSignalFocus || reason != "" {
		t.Fatalf("expected focus hunk, got %s/%q", signal, reason)
	}
}

func TestClassifyAuditPathReasons(t *testing.T) {
	hunk := DiffHunk{Lines: []DiffLine{{Type: LineAdded, Content: "x"}}}
	tests := []struct {
		path   string
		reason string
	}{
		{"generated/client.go", "generated"},
		{"vendor/lib.go", "vendor"},
		{"go.sum", "lockfile"},
	}
	for _, tt := range tests {
		signal, reason := ClassifyHunk(tt.path, hunk)
		if signal != HunkSignalAudit || reason != tt.reason {
			t.Fatalf("%s: expected audit/%s, got %s/%q", tt.path, tt.reason, signal, reason)
		}
	}
}

func TestClassifyMixedImportAndLogicAsFocus(t *testing.T) {
	hunk := DiffHunk{Lines: []DiffLine{
		{Type: LineAdded, Content: "\t\"strings\""},
		{Type: LineDeleted, Content: "return oldValue"},
		{Type: LineAdded, Content: "return newValue"},
	}}
	signal, reason := ClassifyHunk("main.go", hunk)
	if signal != HunkSignalFocus || reason != "" {
		t.Fatalf("expected mixed hunk to stay focus, got %s/%q", signal, reason)
	}
}
