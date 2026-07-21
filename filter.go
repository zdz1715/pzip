package pzip

import (
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

type Matcher interface {
	Match(pattern, name string) bool
}

type Filter struct {
	Include []string
	Exclude []string

	Matcher Matcher
}

func (f Filter) Accept(name string) bool {
	if len(f.Include) > 0 && !f.matchAny(f.Include, name) {
		return false
	}
	if len(f.Exclude) > 0 && f.matchAny(f.Exclude, name) {
		return false
	}
	return true
}

func (f Filter) matchAny(patterns []string, name string) bool {
	matcher := f.Matcher
	if matcher == nil {
		matcher = DefaultMatcher{}
	}
	for _, pattern := range patterns {
		if matcher.Match(pattern, name) {
			return true
		}
	}
	return false
}

func NewFilter(includes, excludes []string, matcher ...Matcher) Filter {
	var m Matcher
	if len(matcher) > 0 && matcher[0] != nil {
		m = matcher[0]
	} else {
		m = newGlobMatcher(includes, excludes)
	}
	return Filter{
		Include: includes,
		Exclude: excludes,
		Matcher: m,
	}
}

type DefaultMatcher struct{}

func (DefaultMatcher) Match(pattern, name string) bool {
	g, err := compileGlob(pattern)
	return err == nil && g.Match(filepath.ToSlash(name))
}

type globMatcher struct {
	patterns map[string]glob.Glob
}

func newGlobMatcher(patternSets ...[]string) *globMatcher {
	matcher := &globMatcher{patterns: make(map[string]glob.Glob)}
	for _, patterns := range patternSets {
		for _, pattern := range patterns {
			if _, ok := matcher.patterns[pattern]; ok {
				continue
			}
			g, err := compileGlob(pattern)
			if err == nil {
				matcher.patterns[pattern] = g
			} else {
				matcher.patterns[pattern] = nil
			}
		}
	}
	return matcher
}

func (m *globMatcher) Match(pattern, name string) bool {
	g, ok := m.patterns[pattern]
	if !ok {
		return DefaultMatcher{}.Match(pattern, name)
	}
	return g != nil && g.Match(filepath.ToSlash(name))
}

func compileGlob(pattern string) (glob.Glob, error) {
	return glob.Compile(normalizeInfoZIPPattern(pattern))
}

func normalizeInfoZIPPattern(pattern string) string {
	if !strings.Contains(pattern, "[^") {
		return pattern
	}

	var normalized strings.Builder
	normalized.Grow(len(pattern))
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			normalized.WriteByte(pattern[i])
			i++
			normalized.WriteByte(pattern[i])
			continue
		}
		if pattern[i] == '[' && i+1 < len(pattern) && pattern[i+1] == '^' {
			normalized.WriteString("[!")
			i++
			continue
		}
		normalized.WriteByte(pattern[i])
	}
	return normalized.String()
}
