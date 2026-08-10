package jenkins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
	"github.com/vanilla-x/pluget/internal/match"
	"github.com/vanilla-x/pluget/internal/source"
)

// Resolver downloads build artifacts from Jenkins.
type Resolver struct {
	Client *download.Client
}

type jobInfo struct {
	Builds []struct {
		Number int `json:"number"`
	} `json:"builds"`
}

type buildInfo struct {
	Number    int        `json:"number"`
	Result    string     `json:"result"`
	Artifacts []artifact `json:"artifacts"`
}

type artifact struct {
	FileName     string `json:"fileName"`
	RelativePath string `json:"relativePath"`
}

// Resolve implements source.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p config.Plugin) (source.Artifact, error) {
	host := strings.TrimRight(p.Host, "/")
	jobPath := jobAPIPath(p.Job)

	if p.Build != "" {
		return r.resolveBuild(ctx, host, jobPath, p.Build, p.Artifact, p.Job)
	}

	builds, err := r.listBuildNumbers(ctx, host, jobPath)
	if err != nil {
		return source.Artifact{}, err
	}
	for _, num := range builds {
		art, err := r.resolveBuild(ctx, host, jobPath, strconv.Itoa(num), p.Artifact, p.Job)
		if err != nil {
			// build may lack artifacts or not match — keep searching farther back
			continue
		}
		return art, nil
	}
	return source.Artifact{}, fmt.Errorf("no build with matching artifact %q for job %s", p.Artifact, p.Job)
}

func (r *Resolver) resolveBuild(ctx context.Context, host, jobPath, build, artifactPattern, jobName string) (source.Artifact, error) {
	u := fmt.Sprintf("%s/%s/%s/api/json", host, jobPath, url.PathEscape(build))
	resp, err := r.Client.Get(ctx, u, nil)
	if err != nil {
		return source.Artifact{}, fmt.Errorf("jenkins build %s: %w", build, err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return source.Artifact{}, err
	}
	if resp.StatusCode != 200 {
		return source.Artifact{}, fmt.Errorf("jenkins build %s: HTTP %d: %s", build, resp.StatusCode, truncate(body))
	}
	var info buildInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return source.Artifact{}, fmt.Errorf("jenkins decode: %w", err)
	}
	for _, a := range info.Artifacts {
		if !match.ArtifactMatches(artifactPattern, a.FileName) {
			continue
		}
		dl := fmt.Sprintf("%s/%s/%d/artifact/%s", host, jobPath, info.Number, a.RelativePath)
		return source.Artifact{
			URL:      dl,
			Filename: a.FileName,
			Label:    fmt.Sprintf("jenkins %s#%d %s", jobName, info.Number, a.FileName),
		}, nil
	}
	return source.Artifact{}, fmt.Errorf("no artifact matching %q in build %s", artifactPattern, build)
}

func (r *Resolver) listBuildNumbers(ctx context.Context, host, jobPath string) ([]int, error) {
	u := fmt.Sprintf("%s/%s/api/json", host, jobPath)
	resp, err := r.Client.Get(ctx, u, nil)
	if err != nil {
		return nil, fmt.Errorf("jenkins list builds: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("jenkins list builds: HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var info jobInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("jenkins decode: %w", err)
	}
	nums := make([]int, 0, len(info.Builds))
	for _, b := range info.Builds {
		nums = append(nums, b.Number)
	}
	return nums, nil
}

// jobAPIPath converts "Job" or "Folder/Job" into Jenkins URL path segments.
func jobAPIPath(job string) string {
	parts := strings.Split(strings.Trim(job, "/"), "/")
	escaped := make([]string, 0, len(parts)*2)
	for _, p := range parts {
		if p == "" {
			continue
		}
		escaped = append(escaped, "job", url.PathEscape(p))
	}
	return strings.Join(escaped, "/")
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
