package github

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type PRSearchResult struct {
	Owner     string
	Repo      string
	Number    int
	Title     string
	URL       string
	Author    string
	UpdatedAt time.Time
	IsDraft   bool
}

func (p PRSearchResult) RepoName() string {
	if p.Owner == "" || p.Repo == "" {
		return ""
	}
	return p.Owner + "/" + p.Repo
}

func (p PRSearchResult) Target() Target {
	repo := p.RepoName()
	return Target{
		Raw:    fmt.Sprintf("%s#%d", repo, p.Number),
		Owner:  p.Owner,
		Repo:   p.Repo,
		Number: p.Number,
	}
}

type RepositorySearchResult struct {
	FullName    string
	Description string
	UpdatedAt   time.Time
	PushedAt    time.Time
	IsPrivate   bool
}

const repositoryPoolQuery = `query {
  viewer {
    repositories(first: 100, ownerAffiliations: [OWNER, COLLABORATOR, ORGANIZATION_MEMBER], orderBy: {field: UPDATED_AT, direction: DESC}) {
      nodes {
        nameWithOwner
        description
        isPrivate
        isArchived
        pushedAt
        updatedAt
      }
    }
    repositoriesContributedTo(first: 100, includeUserRepositories: true, contributionTypes: [COMMIT, PULL_REQUEST, REPOSITORY]) {
      nodes {
        nameWithOwner
        description
        isPrivate
        isArchived
        pushedAt
        updatedAt
      }
    }
  }
}`

func (c Client) SearchReviewRequests(ctx context.Context) ([]PRSearchResult, error) {
	args := []string{
		"search", "prs",
		"--review-requested=@me",
		"--state=open",
		"--sort=updated",
		"--order=desc",
		"--json", "number,title,url,updatedAt,repository,author,isDraft",
		"-L", "50",
	}
	return c.searchPRs(ctx, args)
}

func (c Client) ListOpenPRs(ctx context.Context, ownerRepo string) ([]PRSearchResult, error) {
	ownerRepo = strings.TrimSpace(ownerRepo)
	if ownerRepo == "" {
		return nil, fmt.Errorf("repository is required")
	}
	args := []string{
		"search", "prs",
		"--repo", ownerRepo,
		"--state=open",
		"--sort=updated",
		"--order=desc",
		"--json", "number,title,url,updatedAt,author,isDraft",
		"-L", "50",
	}
	results, err := c.searchPRs(ctx, args)
	if err != nil {
		return nil, err
	}
	owner, repo, ok := strings.Cut(ownerRepo, "/")
	if !ok {
		return results, nil
	}
	for i := range results {
		if results[i].Owner == "" {
			results[i].Owner = owner
		}
		if results[i].Repo == "" {
			results[i].Repo = repo
		}
	}
	return results, nil
}

func (c Client) LoadRepositoryPool(ctx context.Context) ([]RepositorySearchResult, error) {
	out, err := c.outputGh(ctx, "api", "graphql", "-f", "query="+repositoryPoolQuery)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data struct {
			Viewer struct {
				Repositories struct {
					Nodes []repositoryGraphQLNode `json:"nodes"`
				} `json:"repositories"`
				RepositoriesContributedTo struct {
					Nodes []repositoryGraphQLNode `json:"nodes"`
				} `json:"repositoriesContributedTo"`
			} `json:"viewer"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("could not parse gh repository pool output: %w", err)
	}
	repos := map[string]RepositorySearchResult{}
	add := func(nodes []repositoryGraphQLNode) {
		for _, node := range nodes {
			if node.IsArchived || node.NameWithOwner == "" {
				continue
			}
			repo := RepositorySearchResult{
				FullName:    node.NameWithOwner,
				Description: node.Description,
				UpdatedAt:   parseGitHubTime(node.UpdatedAt),
				PushedAt:    parseGitHubTime(node.PushedAt),
				IsPrivate:   node.IsPrivate,
			}
			existing, ok := repos[repo.FullName]
			if !ok || repoActivity(repo).After(repoActivity(existing)) {
				repos[repo.FullName] = repo
			}
		}
	}
	add(payload.Data.Viewer.Repositories.Nodes)
	add(payload.Data.Viewer.RepositoriesContributedTo.Nodes)

	results := make([]RepositorySearchResult, 0, len(repos))
	for _, repo := range repos {
		results = append(results, repo)
	}
	sortRepositories(results)
	return results, nil
}

type repositoryGraphQLNode struct {
	NameWithOwner string `json:"nameWithOwner"`
	Description   string `json:"description"`
	IsPrivate     bool   `json:"isPrivate"`
	IsArchived    bool   `json:"isArchived"`
	UpdatedAt     string `json:"updatedAt"`
	PushedAt      string `json:"pushedAt"`
}

func FilterRepositories(pool []RepositorySearchResult, query string) []RepositorySearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	needle := strings.ToLower(query)
	results := make([]RepositorySearchResult, 0, len(pool))
	for _, repo := range pool {
		haystack := strings.ToLower(repo.FullName + " " + repo.Description)
		if strings.Contains(haystack, needle) {
			results = append(results, repo)
		}
	}
	sortRepositories(results)
	return results
}

func (c Client) searchPRs(ctx context.Context, args []string) ([]PRSearchResult, error) {
	out, err := c.outputGh(ctx, args...)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		URL       string `json:"url"`
		UpdatedAt string `json:"updatedAt"`
		IsDraft   bool   `json:"isDraft"`
		Author    struct {
			Login string `json:"login"`
		} `json:"author"`
		Repository struct {
			NameWithOwner string `json:"nameWithOwner"`
			FullName      string `json:"fullName"`
			Name          string `json:"name"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return nil, fmt.Errorf("could not parse gh search prs output: %w", err)
	}
	results := make([]PRSearchResult, 0, len(payload))
	for _, pr := range payload {
		repoName := pr.Repository.NameWithOwner
		if repoName == "" {
			repoName = pr.Repository.FullName
		}
		if repoName == "" && pr.Repository.Owner.Login != "" && pr.Repository.Name != "" {
			repoName = pr.Repository.Owner.Login + "/" + pr.Repository.Name
		}
		owner, repo := splitRepoName(repoName)
		results = append(results, PRSearchResult{
			Owner:     owner,
			Repo:      repo,
			Number:    pr.Number,
			Title:     pr.Title,
			URL:       pr.URL,
			Author:    pr.Author.Login,
			UpdatedAt: parseGitHubTime(pr.UpdatedAt),
			IsDraft:   pr.IsDraft,
		})
	}
	return results, nil
}

func splitRepoName(name string) (string, string) {
	owner, repo, ok := strings.Cut(name, "/")
	if !ok {
		return "", ""
	}
	return owner, repo
}

func parseGitHubTime(raw string) time.Time {
	t, _ := time.Parse(time.RFC3339, raw)
	return t
}

func sortRepositories(repos []RepositorySearchResult) {
	sort.SliceStable(repos, func(i, j int) bool {
		left := repoActivity(repos[i])
		right := repoActivity(repos[j])
		if left.Equal(right) {
			return repos[i].FullName < repos[j].FullName
		}
		return left.After(right)
	})
}

func repoActivity(repo RepositorySearchResult) time.Time {
	if !repo.PushedAt.IsZero() {
		return repo.PushedAt
	}
	return repo.UpdatedAt
}
