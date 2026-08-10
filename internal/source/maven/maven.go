package maven

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
	"github.com/vanilla-x/pluget/internal/match"
	"github.com/vanilla-x/pluget/internal/source"
)

const defaultHost = "https://repo1.maven.org/maven2/"

// Resolver downloads artifacts from a Maven repository.
type Resolver struct {
	Client *download.Client
}

type metadata struct {
	Versioning struct {
		Versions []string `xml:"versions>version"`
		Release  string   `xml:"release"`
		Latest   string   `xml:"latest"`
	} `xml:"versioning"`
}

// Resolve implements source.Resolver.
func (r *Resolver) Resolve(ctx context.Context, p config.Plugin) (source.Artifact, error) {
	host := strings.TrimSpace(p.Host)
	if host == "" {
		host = defaultHost
	}
	if !strings.HasSuffix(host, "/") {
		host += "/"
	}

	groupPath := strings.ReplaceAll(p.Group, ".", "/")
	metaURL := host + groupPath + "/" + p.Artifact + "/maven-metadata.xml"

	versions, err := r.listVersions(ctx, metaURL)
	if err != nil {
		return source.Artifact{}, err
	}

	best := match.MaxMatching(p.Version, versions)
	if best == "" {
		// Also try exact/range against listed order (newest last in metadata often);
		// MaxMatching already picks highest. Fall back: walk reverse for first match.
		for i := len(versions) - 1; i >= 0; i-- {
			if match.VersionMatches(p.Version, versions[i]) {
				best = versions[i]
				break
			}
		}
	}
	if best == "" {
		return source.Artifact{}, fmt.Errorf("no matching version for %s:%s (version=%q)", p.Group, p.Artifact, p.Version)
	}

	filename := fmt.Sprintf("%s-%s.jar", p.Artifact, best)
	dl := host + groupPath + "/" + p.Artifact + "/" + url.PathEscape(best) + "/" + filename
	return source.Artifact{
		URL:      dl,
		Filename: filename,
		Label:    fmt.Sprintf("maven %s:%s:%s", p.Group, p.Artifact, best),
	}, nil
}

func (r *Resolver) listVersions(ctx context.Context, metaURL string) ([]string, error) {
	resp, err := r.Client.Get(ctx, metaURL, map[string]string{"Accept": "application/xml, text/xml, */*"})
	if err != nil {
		return nil, fmt.Errorf("maven metadata: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("maven metadata: HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var md metadata
	if err := xml.Unmarshal(body, &md); err != nil {
		return nil, fmt.Errorf("maven metadata decode: %w", err)
	}
	if len(md.Versioning.Versions) == 0 {
		return nil, fmt.Errorf("maven metadata: no versions at %s", metaURL)
	}
	return md.Versioning.Versions, nil
}

func truncate(b []byte) string {
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
