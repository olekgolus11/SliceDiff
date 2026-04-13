package github

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Target struct {
	Raw    string
	Owner  string
	Repo   string
	Number int
	IsURL  bool
}

var ownerRepoNumberRE = regexp.MustCompile(`^([^/\s]+)/([^#\s]+)#([0-9]+)$`)

func ParseTarget(raw string) (Target, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Target{}, fmt.Errorf("target is required")
	}

	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return Target{Raw: raw, Number: n}, nil
	}

	if match := ownerRepoNumberRE.FindStringSubmatch(raw); match != nil {
		n, _ := strconv.Atoi(match[3])
		return Target{Raw: raw, Owner: match[1], Repo: match[2], Number: n}, nil
	}

	parsed, err := url.Parse(raw)
	if err == nil && parsed.Host == "github.com" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) == 4 && parts[2] == "pull" {
			n, err := strconv.Atoi(parts[3])
			if err != nil || n <= 0 {
				return Target{}, fmt.Errorf("invalid pull request number in %q", raw)
			}
			return Target{Raw: raw, Owner: parts[0], Repo: parts[1], Number: n, IsURL: true}, nil
		}
	}

	return Target{}, fmt.Errorf("expected PR URL, owner/repo#number, or PR number")
}

func (t Target) PRArg() string {
	if t.IsURL {
		return t.Raw
	}
	return strconv.Itoa(t.Number)
}

func (t Target) RepoArg() string {
	if t.Owner == "" || t.Repo == "" {
		return ""
	}
	return t.Owner + "/" + t.Repo
}
