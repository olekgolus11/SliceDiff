package diff

import "testing"

func TestParseUnifiedModifiedFile(t *testing.T) {
	raw := `diff --git a/main.go b/main.go
index 1111111..2222222 100644
--- a/main.go
+++ b/main.go
@@ -1,2 +1,2 @@
 package main
-old
+new
`
	files, err := ParseUnified(raw)
	if err != nil {
		t.Fatalf("ParseUnified returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	file := files[0]
	if file.Path != "main.go" || file.Status != "modified" {
		t.Fatalf("unexpected file: %+v", file)
	}
	if len(file.Hunks) != 1 || file.Hunks[0].ID != "h1" {
		t.Fatalf("unexpected hunks: %+v", file.Hunks)
	}
	if got := file.Hunks[0].Lines[1].Type; got != LineDeleted {
		t.Fatalf("expected deleted line, got %s", got)
	}
	if got := file.Hunks[0].Lines[2].Type; got != LineAdded {
		t.Fatalf("expected added line, got %s", got)
	}
}

func TestParseUnifiedAddedDeletedRenamedAndBinary(t *testing.T) {
	raw := `diff --git a/new.go b/new.go
new file mode 100644
--- /dev/null
+++ b/new.go
@@ -0,0 +1 @@
+package main
diff --git a/old.go b/old.go
deleted file mode 100644
--- a/old.go
+++ /dev/null
@@ -1 +0,0 @@
-package main
diff --git a/old_name.go b/new_name.go
similarity index 90%
rename from old_name.go
rename to new_name.go
--- a/old_name.go
+++ b/new_name.go
@@ -1 +1 @@
-old
+new
diff --git a/image.png b/image.png
Binary files a/image.png and b/image.png differ
`
	files, err := ParseUnified(raw)
	if err != nil {
		t.Fatalf("ParseUnified returned error: %v", err)
	}
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d", len(files))
	}
	if files[0].Status != "added" {
		t.Fatalf("expected added, got %+v", files[0])
	}
	if files[1].Status != "deleted" {
		t.Fatalf("expected deleted, got %+v", files[1])
	}
	if files[2].Status != "renamed" || files[2].OldPath != "old_name.go" || files[2].Path != "new_name.go" {
		t.Fatalf("expected renamed, got %+v", files[2])
	}
	if !files[3].IsBinary {
		t.Fatalf("expected binary file, got %+v", files[3])
	}
}

func TestIsGeneratedPath(t *testing.T) {
	for _, path := range []string{"generated/client.go", "package-lock.json", "vendor/lib.go"} {
		if !IsGeneratedPath(path) {
			t.Fatalf("expected %s to be generated/lockfile", path)
		}
	}
	if IsGeneratedPath("internal/app/app.go") {
		t.Fatal("did not expect normal source path to be generated")
	}
}
