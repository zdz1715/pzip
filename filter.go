package pzip

import (
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"
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
		m = DefaultMatcher{}
	}
	return Filter{
		Include: includes,
		Exclude: excludes,
		Matcher: m,
	}
}

type DefaultMatcher struct{}

func (DefaultMatcher) Match(pattern, name string) bool {
	ok, _ := doublestar.Match(pattern, filepath.ToSlash(name))
	return ok
}
