package agent

import (
	"encoding/json"
	"os"
	"testing"
)

func TestWriteSchemaFileIncludesTypesForConstFields(t *testing.T) {
	path, cleanup, err := writeSchemaFile()
	if err != nil {
		t.Fatalf("writeSchemaFile returned error: %v", err)
	}
	defer cleanup()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read schema file: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties missing or invalid: %#v", schema["properties"])
	}
	schemaVersion, ok := properties["schema_version"].(map[string]any)
	if !ok {
		t.Fatalf("schema_version property missing or invalid: %#v", properties["schema_version"])
	}
	if schemaVersion["type"] != "string" {
		t.Fatalf("schema_version must declare type string, got %#v", schemaVersion["type"])
	}
	if schemaVersion["const"] != SchemaVersion {
		t.Fatalf("schema_version const mismatch: %#v", schemaVersion["const"])
	}

	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("schema defs missing or invalid: %#v", schema["$defs"])
	}
	sliceDef, ok := defs["slice"].(map[string]any)
	if !ok {
		t.Fatalf("slice definition missing or invalid: %#v", defs["slice"])
	}
	required, ok := sliceDef["required"].([]any)
	if !ok {
		t.Fatalf("slice required fields missing or invalid: %#v", sliceDef["required"])
	}
	if !containsString(required, "reading_steps") {
		t.Fatalf("slice schema must require reading_steps, got %#v", required)
	}
	stepDef, ok := defs["reading_step"].(map[string]any)
	if !ok {
		t.Fatalf("reading_step definition missing or invalid: %#v", defs["reading_step"])
	}
	stepRequired, ok := stepDef["required"].([]any)
	if !ok {
		t.Fatalf("reading_step required fields missing or invalid: %#v", stepDef["required"])
	}
	if !containsString(stepRequired, "body") {
		t.Fatalf("reading_step schema must require body, got %#v", stepRequired)
	}
}

func containsString(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
