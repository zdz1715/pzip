package pzip

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bmatcuk/doublestar/v4"
)

type SkipPath struct {
	Includes []string
	Excludes []string
}

func (p SkipPath) SkipOnSlash(path string) bool {
	if len(p.Includes) > 0 {
		in := false
		for _, pattern := range p.Includes {
			ok, _ := doublestar.Match(pattern, path)
			if ok {
				in = true
				break
			}
		}
		if !in {
			return true
		}
	}
	for _, pattern := range p.Excludes {
		ok, _ := doublestar.Match(pattern, path)
		if ok {
			return true
		}
	}

	return false
}

func (p SkipPath) Skip(path string) bool {
	if len(p.Includes) > 0 {
		in := false
		for _, pattern := range p.Includes {
			ok, _ := doublestar.PathMatch(pattern, path)
			if ok {
				in = true
				break
			}
		}
		if !in {
			return true
		}
	}
	for _, pattern := range p.Excludes {
		ok, _ := doublestar.PathMatch(pattern, path)
		if ok {
			return true
		}
	}

	return false
}

func SetupSignalContext() context.Context {
	shutdownHandler := make(chan os.Signal, 2)
	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(shutdownHandler, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	go func() {
		s := <-shutdownHandler
		_, _ = fmt.Fprintf(os.Stderr, "\nReceived signal: %s, stopping...\n", s.String())
		cancel()
		<-shutdownHandler
		os.Exit(1) // second signal. Exit directly.
	}()
	return ctx
}
