package github

import "testing"

func TestParseTargetURL(t *testing.T) {
	target, err := ParseTarget("https://github.com/owner/repo/pull/42")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if !target.IsURL || target.Owner != "owner" || target.Repo != "repo" || target.Number != 42 {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestParseTargetOwnerRepoNumber(t *testing.T) {
	target, err := ParseTarget("owner/repo#123")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.RepoArg() != "owner/repo" || target.PRArg() != "123" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestParseTargetNumber(t *testing.T) {
	target, err := ParseTarget("7")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.Number != 7 || target.RepoArg() != "" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestParseTargetRejectsInvalid(t *testing.T) {
	if _, err := ParseTarget("owner/repo"); err == nil {
		t.Fatal("expected invalid target error")
	}
}
