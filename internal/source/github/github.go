package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
	"github.com/vanilla-x/pluget/internal/match"
	"github.com/vanilla-x/pluget/internal/source"
)

const apiBase = "https://api.github.com"

// Resolver downloads assets from GitHub Releases.
type Resolver struct {
	Client *download.Client
}

type release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Resolve implements source.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p config.Plugin) (source.Artifact, error) {
	parts := strings.SplitN(p.Repository, "/", 2)
	if len(parts) != 2 {
		return source.Artifact{}, fmt.Errorf("invalid repository %q", p.Repository)
	}
	owner, repo := parts[0], parts[1]

	headers := authHeaders()
	releases, err := r.listReleases(ctx, owner, repo, headers)
	if err != nil {
		return source.Artifact{}, err
	}

	for _, rel := range releases {
		if rel.Draft {
			continue
		}
		verOK := match.VersionMatches(p.Version, rel.TagName) ||
			(rel.Name != "" && match.VersionMatches(p.Version, rel.Name))
		if !verOK {
			continue
		}
		for _, a := range rel.Assets {
			if !match.ArtifactMatches(p.Artifact, a.Name) {
				continue
			}
			return source.Artifact{
				URL:      a.BrowserDownloadURL,
				Filename: a.Name,
				Headers:  headers,
				Label:    fmt.Sprintf("github %s@%s", p.Repository, rel.TagName),
			}, nil
		}
	}
	return source.Artifact{}, fmt.Errorf("no matching release asset for %s (version=%q artifact=%q)",
		p.Repository, p.Version, p.Artifact)
}

func (r *Resolver) listReleases(ctx context.Context, owner, repo string, headers map[string]string) ([]release, error) {
	var all []release
	page := 1
	const perPage = 100
	for {
		u := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=%d&page=%d", apiBase, owner, repo, perPage, page)
		resp, err := r.Client.Get(ctx, u, headers)
		if err != nil {
			return nil, fmt.Errorf("github list releases: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("github list releases: HTTP %d: %s", resp.StatusCode, truncate(body))
		}
		var batch []release
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("github decode: %w", err)
		}
		all = append(all, batch...)
		if len(batch) < perPage {
			break
		}
		page++
	}
	return all, nil
}

func authHeaders() map[string]string {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	h := map[string]string{
		"Accept": "application/vnd.github+json",
	}
	if token != "" {
		h["Authorization"] = "Bearer " + token
	}
	return h
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
