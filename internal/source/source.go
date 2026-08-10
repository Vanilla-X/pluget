package source

import (
	"context"
	"fmt"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
)

// Artifact is a resolved downloadable file.
type Artifact struct {
	URL      string
	Filename string
	Headers  map[string]string
	Label    string // human description of what was selected
}

// Resolver resolves a plugin config entry to a downloadable artifact.
type Resolver interface {
	Resolve(ctx context.Context, p config.Plugin) (Artifact, error)
}

// Registry maps source names to resolvers.
type Registry struct {
	resolvers map[string]Resolver
}

// NewRegistry builds an empty source registry. Call Register for each source.
func NewRegistry(_ *download.Client) *Registry {
	return &Registry{
		resolvers: map[string]Resolver{},
	}
}

// Register adds a resolver for a source name.
func (r *Registry) Register(name string, resolver Resolver) {
	r.resolvers[name] = resolver
}

// Resolve dispatches to the appropriate source resolver.
func (r *Registry) Resolve(ctx context.Context, p config.Plugin) (Artifact, error) {
	res, ok := r.resolvers[p.Source]
	if !ok {
		return Artifact{}, fmt.Errorf("no resolver for source %q", p.Source)
	}
	return res.Resolve(ctx, p)
}
