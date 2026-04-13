package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/olekgolus11/SliceDiff/internal/diff"
)

type PullRequest struct {
	Owner      string          `json:"owner"`
	Repo       string          `json:"repo"`
	Number     int             `json:"number"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	Author     string          `json:"author"`
	BaseBranch string          `json:"base_branch"`
	HeadBranch string          `json:"head_branch"`
	HeadSHA    string          `json:"head_sha"`
	Files      []diff.DiffFile `json:"files"`
	RawDiff    string          `json:"raw_diff"`
}

type Client struct {
	Timeout time.Duration
}

func NewClient() Client {
	return Client{Timeout: 30 * time.Second}
}

func (c Client) CheckAuth(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found on PATH; install GitHub CLI and run gh auth login")
	}
	if err := c.run(ctx, nil, "auth", "status"); err != nil {
		return fmt.Errorf("GitHub CLI is not authenticated or cannot access GitHub; run gh auth login: %w", err)
	}
	return nil
}

func (c Client) Fetch(ctx context.Context, target Target) (*PullRequest, error) {
	if err := c.CheckAuth(ctx); err != nil {
		return nil, err
	}

	repo := target.RepoArg()
	if repo == "" {
		resolved, err := c.currentRepo(ctx)
		if err != nil {
			return nil, err
		}
		repo = resolved
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			target.Owner = parts[0]
			target.Repo = parts[1]
		}
	}

	meta, err := c.prView(ctx, target, repo)
	if err != nil {
		return nil, err
	}
	rawDiff, err := c.prDiff(ctx, target, repo)
	if err != nil {
		return nil, err
	}
	files, err := diff.ParseUnified(rawDiff)
	if err != nil {
		return nil, err
	}

	return &PullRequest{
		Owner:      target.Owner,
		Repo:       target.Repo,
		Number:     meta.Number,
		Title:      meta.Title,
		Body:       meta.Body,
		Author:     meta.Author.Login,
		BaseBranch: meta.BaseRefName,
		HeadBranch: meta.HeadRefName,
		HeadSHA:    meta.HeadRefOID,
		Files:      files,
		RawDiff:    rawDiff,
	}, nil
}

type prViewResponse struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	HeadRefOID  string `json:"headRefOid"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

func (c Client) prView(ctx context.Context, target Target, repo string) (prViewResponse, error) {
	args := []string{"pr", "view", target.PRArg(), "--json", "number,title,body,author,baseRefName,headRefName,headRefOid"}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	out, err := c.output(ctx, args...)
	if err != nil {
		return prViewResponse{}, err
	}
	var meta prViewResponse
	if err := json.Unmarshal(out, &meta); err != nil {
		return prViewResponse{}, fmt.Errorf("could not parse gh pr view output: %w", err)
	}
	return meta, nil
}

func (c Client) prDiff(ctx context.Context, target Target, repo string) (string, error) {
	args := []string{"pr", "diff", target.PRArg()}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	out, err := c.output(ctx, args...)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (c Client) currentRepo(ctx context.Context) (string, error) {
	out, err := c.output(ctx, "repo", "view", "--json", "nameWithOwner")
	if err != nil {
		return "", err
	}
	var payload struct {
		NameWithOwner string `json:"nameWithOwner"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return "", fmt.Errorf("could not parse gh repo view output: %w", err)
	}
	if payload.NameWithOwner == "" {
		return "", fmt.Errorf("could not resolve current GitHub repository")
	}
	return payload.NameWithOwner, nil
}

func (c Client) output(ctx context.Context, args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), msg)
	}
	return out, nil
}

func (c Client) run(ctx context.Context, stdin []byte, args ...string) error {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return errors.New(msg)
	}
	return nil
}
