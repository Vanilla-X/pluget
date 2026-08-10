package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync/atomic"

	"golang.org/x/sync/errgroup"

	"github.com/vanilla-x/pluget/internal/config"
	"github.com/vanilla-x/pluget/internal/download"
	"github.com/vanilla-x/pluget/internal/logx"
	"github.com/vanilla-x/pluget/internal/source"
	ghsrc "github.com/vanilla-x/pluget/internal/source/github"
	"github.com/vanilla-x/pluget/internal/source/hangar"
	"github.com/vanilla-x/pluget/internal/source/jenkins"
	"github.com/vanilla-x/pluget/internal/source/maven"
	"github.com/vanilla-x/pluget/internal/source/modrinth"
)

const defaultConcurrency = 8

func main() {
	os.Exit(run())
}

func run() int {
	configPath := flag.String("config", "", "path to config.yml")
	outPath := flag.String("out", "", "directory to download plugins into")
	flag.Parse()

	if *configPath == "" || *outPath == "" {
		fmt.Fprintf(os.Stderr, "usage: pluget -config <path> -out <path>\n")
		flag.PrintDefaults()
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		logx.Errorf("%v", err)
		return 1
	}

	client := download.NewClient()
	reg := source.NewRegistry(client)
	reg.Register(config.SourceModrinth, &modrinth.Resolver{Client: client})
	reg.Register(config.SourceHangar, &hangar.Resolver{Client: client})
	reg.Register(config.SourceGitHubReleases, &ghsrc.Resolver{Client: client})
	reg.Register(config.SourceJenkins, &jenkins.Resolver{Client: client})
	reg.Register(config.SourceMaven, &maven.Resolver{Client: client})

	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(defaultConcurrency)

	var failures atomic.Int32

	for i := range cfg.Plugins {
		p := cfg.Plugins[i]
		g.Go(func() error {
			logx.Infof("resolving %s", p.Label())
			art, err := reg.Resolve(ctx, p)
			if err != nil {
				logx.Warnf("%s: %v", p.Label(), err)
				failures.Add(1)
				return nil
			}
			logx.Infof("downloading %s", art.Label)
			dest, err := client.ToFile(ctx, art.URL, *outPath, art.Filename, art.Headers)
			if err != nil {
				logx.Warnf("%s: download failed: %v", p.Label(), err)
				failures.Add(1)
				return nil
			}
			logx.Infof("saved %s", dest)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		logx.Errorf("%v", err)
		return 1
	}
	if n := failures.Load(); n > 0 {
		logx.Errorf("%d plugin(s) failed", n)
		return 1
	}
	return 0
}
