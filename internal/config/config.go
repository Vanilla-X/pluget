package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SourceModrinth       = "modrinth"
	SourceHangar         = "hangar"
	SourceJenkins        = "jenkins"
	SourceGitHubReleases = "github-releases"
	SourceMaven          = "maven"
)

// Config is the root YAML configuration.
type Config struct {
	Plugins []Plugin `yaml:"plugins"`
}

// Plugin is a single download entry. Fields are source-specific; unused fields are ignored.
type Plugin struct {
	Source string `yaml:"source"`

	// Modrinth / Hangar
	ID       string `yaml:"id"`
	Platform string `yaml:"platform"`

	// Shared version constraint (exact, *, or Maven range)
	Version string `yaml:"version"`

	// Jenkins
	Host  string `yaml:"host"`
	Job   string `yaml:"job"`
	Build string `yaml:"build"`

	// GitHub Releases / Jenkins / Maven filename pattern
	Artifact string `yaml:"artifact"`

	// GitHub Releases
	Repository string `yaml:"repository"`

	// Maven
	Group string `yaml:"group"`
}

// Load reads and validates a YAML config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Plugins) == 0 {
		return nil, fmt.Errorf("config has no plugins")
	}
	for i := range cfg.Plugins {
		if err := cfg.Plugins[i].Validate(); err != nil {
			return nil, fmt.Errorf("plugins[%d]: %w", i, err)
		}
	}
	return &cfg, nil
}

// Validate checks required fields for the plugin's source.
func (p *Plugin) Validate() error {
	p.Source = strings.ToLower(strings.TrimSpace(p.Source))
	switch p.Source {
	case SourceModrinth:
		if p.ID == "" {
			return fmt.Errorf("modrinth requires id")
		}
	case SourceHangar:
		if p.ID == "" {
			return fmt.Errorf("hangar requires id")
		}
	case SourceJenkins:
		if p.Host == "" {
			return fmt.Errorf("jenkins requires host")
		}
		if p.Job == "" {
			return fmt.Errorf("jenkins requires job")
		}
		if p.Artifact == "" {
			return fmt.Errorf("jenkins requires artifact")
		}
	case SourceGitHubReleases:
		if p.Repository == "" {
			return fmt.Errorf("github-releases requires repository")
		}
		if !strings.Contains(p.Repository, "/") {
			return fmt.Errorf("github-releases repository must be owner/repo")
		}
		if p.Artifact == "" {
			return fmt.Errorf("github-releases requires artifact")
		}
	case SourceMaven:
		if p.Group == "" {
			return fmt.Errorf("maven requires group")
		}
		if p.Artifact == "" {
			return fmt.Errorf("maven requires artifact")
		}
		if p.Version == "" {
			return fmt.Errorf("maven requires version")
		}
	case "":
		return fmt.Errorf("source is required")
	default:
		return fmt.Errorf("unknown source %q", p.Source)
	}
	return nil
}

// Label returns a short human-readable identifier for logs.
func (p *Plugin) Label() string {
	switch p.Source {
	case SourceModrinth, SourceHangar:
		return fmt.Sprintf("%s:%s", p.Source, p.ID)
	case SourceJenkins:
		return fmt.Sprintf("jenkins:%s/%s", p.Host, p.Job)
	case SourceGitHubReleases:
		return fmt.Sprintf("github-releases:%s", p.Repository)
	case SourceMaven:
		return fmt.Sprintf("maven:%s:%s", p.Group, p.Artifact)
	default:
		return p.Source
	}
}
