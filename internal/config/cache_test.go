package config

import "testing"

func TestBuildSliceCacheKeyStableAndSensitive(t *testing.T) {
	a := BuildSliceCacheKey("owner", "repo", 1, "sha", "codex", "prompt.v1", "0.1.0")
	b := BuildSliceCacheKey("owner", "repo", 1, "sha", "codex", "prompt.v1", "0.1.0")
	c := BuildSliceCacheKey("owner", "repo", 1, "sha2", "codex", "prompt.v1", "0.1.0")

	if a == "" {
		t.Fatal("expected non-empty cache key")
	}
	if a != b {
		t.Fatalf("expected stable cache key, got %s and %s", a, b)
	}
	if a == c {
		t.Fatal("expected head SHA change to invalidate cache key")
	}
}
