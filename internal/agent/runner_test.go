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
}
