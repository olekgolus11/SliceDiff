package github

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestSearchReviewRequestsBuildsGhSearchAndParsesResults(t *testing.T) {
	var gotArgs []string
	client := Client{output: func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(`[{
			"number": 12,
			"title": "Add picker",
			"url": "https://github.com/owner/repo/pull/12",
			"updatedAt": "2026-04-12T10:20:30Z",
			"isDraft": true,
			"author": {"login": "octo"},
			"repository": {"nameWithOwner": "owner/repo"}
		}]`), nil
	}}

	results, err := client.SearchReviewRequests(context.Background())
	if err != nil {
		t.Fatalf("SearchReviewRequests returned error: %v", err)
	}
	wantArgs := []string{"search", "prs", "--review-requested=@me", "--state=open", "--sort=updated", "--order=desc", "--json", "number,title,url,updatedAt,repository,author,isDraft", "-L", "50"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args:\n got %v\nwant %v", gotArgs, wantArgs)
	}
	if len(results) != 1 || results[0].RepoName() != "owner/repo" || results[0].Number != 12 || !results[0].IsDraft {
		t.Fatalf("unexpected results: %+v", results)
	}
	if target := results[0].Target(); target.Raw != "owner/repo#12" || target.Owner != "owner" || target.Repo != "repo" || target.Number != 12 {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestSearchRepositoriesBuildsGhSearchAndParsesResults(t *testing.T) {
	var gotArgs []string
	client := Client{output: func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(`[{
			"fullName": "owner/repo",
			"description": "Terminal app",
			"updatedAt": "2026-04-12T10:20:30Z",
			"pushedAt": "2026-04-13T10:20:30Z",
			"isPrivate": true
		}]`), nil
	}}

	results, err := client.SearchRepositories(context.Background(), " owner/repo ")
	if err != nil {
		t.Fatalf("SearchRepositories returned error: %v", err)
	}
	wantArgs := []string{"search", "repos", "owner/repo", "--sort=updated", "--order=desc", "--archived=false", "--json", "fullName,description,updatedAt,pushedAt,isPrivate", "-L", "30"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("unexpected args:\n got %v\nwant %v", gotArgs, wantArgs)
	}
	if len(results) != 1 || results[0].FullName != "owner/repo" || !results[0].IsPrivate {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestListOpenPRsAddsRepoToTargetsWhenGhOmitsRepository(t *testing.T) {
	var gotArgs []string
	client := Client{output: func(_ context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(`[{
			"number": 7,
			"title": "Fix resize",
			"url": "https://github.com/owner/repo/pull/7",
			"updatedAt": "2026-04-12T10:20:30Z",
			"isDraft": false,
			"author": {"login": "octo"}
		}]`), nil
	}}

	results, err := client.ListOpenPRs(context.Background(), "owner/repo")
	if err != nil {
		t.Fatalf("ListOpenPRs returned error: %v", err)
	}
	if !strings.Contains(strings.Join(gotArgs, " "), "--repo owner/repo") {
		t.Fatalf("expected repo arg in %v", gotArgs)
	}
	if len(results) != 1 || results[0].RepoName() != "owner/repo" || results[0].Target().Raw != "owner/repo#7" {
		t.Fatalf("unexpected results: %+v", results)
	}
}
