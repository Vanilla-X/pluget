package hangar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
	"github.com/vanilla-x/pluget/internal/match"
	"github.com/vanilla-x/pluget/internal/source"
)

const apiBase = "https://hangar.papermc.io/api/v1"

// Resolver downloads plugins from Hangar.
type Resolver struct {
	Client *download.Client
}

type versionsResponse struct {
	Pagination struct {
		Count  int `json:"count"`
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	} `json:"pagination"`
	Result []hangarVersion `json:"result"`
}

type hangarVersion struct {
	Name      string                     `json:"name"`
	Downloads map[string]platformDownload `json:"downloads"`
}

type platformDownload struct {
	FileInfo    *fileInfo `json:"fileInfo"`
	ExternalURL *string   `json:"externalUrl"`
	DownloadURL *string   `json:"downloadUrl"`
}

type fileInfo struct {
	Name string `json:"name"`
}

// Resolve implements source.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p config.Plugin) (source.Artifact, error) {
	versions, err := r.listVersions(ctx, p.ID)
	if err != nil {
		return source.Artifact{}, err
	}
	wantPlatform := strings.ToUpper(strings.TrimSpace(p.Platform))
	for _, v := range versions {
		if !match.VersionMatches(p.Version, v.Name) {
			continue
		}
		urlStr, filename, ok := pickDownload(v.Downloads, wantPlatform, p.Artifact)
		if !ok {
			continue
		}
		return source.Artifact{
			URL:      urlStr,
			Filename: filename,
			Label:    fmt.Sprintf("hangar %s@%s", p.ID, v.Name),
		}, nil
	}
	return source.Artifact{}, fmt.Errorf("no matching version for %s (version=%q platform=%q)", p.ID, p.Version, p.Platform)
}

func (r *Resolver) listVersions(ctx context.Context, id string) ([]hangarVersion, error) {
	const pageSize = 25
	var all []hangarVersion
	offset := 0
	for {
		u := fmt.Sprintf("%s/projects/%s/versions?limit=%d&offset=%d",
			apiBase, url.PathEscape(id), pageSize, offset)
		resp, err := r.Client.Get(ctx, u, nil)
		if err != nil {
			return nil, fmt.Errorf("hangar list versions: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("hangar list versions: HTTP %d: %s", resp.StatusCode, truncate(body))
		}
		var page versionsResponse
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("hangar decode: %w", err)
		}
		all = append(all, page.Result...)
		offset += len(page.Result)
		if len(page.Result) == 0 || offset >= page.Pagination.Count {
			break
		}
	}
	return all, nil
}

func pickDownload(downloads map[string]platformDownload, platform, artifactPattern string) (urlStr, filename string, ok bool) {
	if len(downloads) == 0 {
		return "", "", false
	}

	try := func(key string, d platformDownload) (string, string, bool) {
		u := ""
		if d.DownloadURL != nil && *d.DownloadURL != "" {
			u = *d.DownloadURL
		} else if d.ExternalURL != nil && *d.ExternalURL != "" {
			u = *d.ExternalURL
		}
		if u == "" {
			return "", "", false
		}
		name := ""
		if d.FileInfo != nil && d.FileInfo.Name != "" {
			name = d.FileInfo.Name
		} else {
			name = baseName(u)
		}
		if artifactPattern != "" && !match.ArtifactMatches(artifactPattern, name) {
			return "", "", false
		}
		return u, name, true
	}

	if platform != "" {
		d, exists := downloads[platform]
		if !exists {
			return "", "", false
		}
		return try(platform, d)
	}

	// Prefer PAPER, then any
	if d, exists := downloads["PAPER"]; exists {
		if u, n, ok := try("PAPER", d); ok {
			return u, n, true
		}
	}
	for k, d := range downloads {
		if u, n, ok := try(k, d); ok {
			return u, n, true
		}
	}
	return "", "", false
}

func baseName(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 && i+1 < len(u) {
		return u[i+1:]
	}
	return u
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
