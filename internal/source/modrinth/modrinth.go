package modrinth

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

const apiBase = "https://api.modrinth.com/v2"

// Resolver downloads plugins from Modrinth.
type Resolver struct {
	Client *download.Client
}

type version struct {
	VersionNumber string   `json:"version_number"`
	Loaders       []string `json:"loaders"`
	Files         []file   `json:"files"`
}

type file struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
}

// Resolve implements source.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p config.Plugin) (source.Artifact, error) {
	versions, err := r.listVersions(ctx, p.ID, p.Platform)
	if err != nil {
		return source.Artifact{}, err
	}
	for _, v := range versions {
		if !match.VersionMatches(p.Version, v.VersionNumber) {
			continue
		}
		f := pickFile(v.Files, p.Artifact)
		if f == nil {
			continue
		}
		return source.Artifact{
			URL:      f.URL,
			Filename: f.Filename,
			Label:    fmt.Sprintf("modrinth %s@%s", p.ID, v.VersionNumber),
		}, nil
	}
	return source.Artifact{}, fmt.Errorf("no matching version for %s (version=%q platform=%q)", p.ID, p.Version, p.Platform)
}

func (r *Resolver) listVersions(ctx context.Context, id, platform string) ([]version, error) {
	const pageSize = 100
	var all []version
	offset := 0
	for {
		u, _ := url.Parse(fmt.Sprintf("%s/project/%s/version", apiBase, url.PathEscape(id)))
		q := u.Query()
		q.Set("limit", fmt.Sprintf("%d", pageSize))
		q.Set("offset", fmt.Sprintf("%d", offset))
		if platform != "" {
			// Modrinth expects a JSON array string for loaders
			enc, _ := json.Marshal([]string{strings.ToLower(platform)})
			q.Set("loaders", string(enc))
		}
		u.RawQuery = q.Encode()

		resp, err := r.Client.Get(ctx, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("modrinth list versions: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("modrinth list versions: HTTP %d: %s", resp.StatusCode, truncate(body))
		}
		var page []version
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("modrinth decode: %w", err)
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break
		}
		offset += pageSize
	}
	return all, nil
}

func pickFile(files []file, artifactPattern string) *file {
	if len(files) == 0 {
		return nil
	}
	if artifactPattern != "" {
		for i := range files {
			if match.ArtifactMatches(artifactPattern, files[i].Filename) {
				return &files[i]
			}
		}
		return nil
	}
	for i := range files {
		if files[i].Primary {
			return &files[i]
		}
	}
	return &files[0]
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
