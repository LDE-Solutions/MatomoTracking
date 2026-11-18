// Package rules compiles the configuration into the tracking rules for each host and path
// and decides whether a request is tracked.
package rules

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/DIE-Bonn/MatomoTracking/internal/config"
)

// Sites holds the rules for every configured host and the top-level fallback.
type Sites struct {
	domains  map[string]*Site // keyed by lower-case host
	fallback *Site
}

// Compile validates the whole configuration, e.g. its path patterns.
func Compile(cfg *config.Config) (*Sites, error) {
	domains := make(map[string]*Site, len(cfg.Domains))
	for host, domain := range cfg.Domains {
		site, err := compileSite(domain)
		if err != nil {
			return nil, fmt.Errorf("domain %q: %w", host, err)
		}
		domains[strings.ToLower(host)] = site
	}

	fallback, err := compileSite(cfg.DefaultDomain())
	if err != nil {
		return nil, err
	}
	return &Sites{domains: domains, fallback: fallback}, nil
}

// For returns the rules for host, falling back to the top-level rules. An explicit domain entry
// wins even if it disables tracking.
func (s *Sites) For(host string) *Site {
	if site, ok := s.domains[strings.ToLower(host)]; ok {
		return site
	}
	return s.fallback
}

// Rules is the effective tracking configuration for a domain or path, with its patterns
// compiled once at startup.
type Rules struct {
	TrackingEnabled    bool
	IdSite             int
	trackedMethods     []string // nil tracks every method
	excludedPaths      []*regexp.Regexp
	includedPaths      []*regexp.Regexp
	responseConditions *config.ResponseConditions
}

// pathRules are the rules for requests below a path prefix.
type pathRules struct {
	prefix string
	rules  Rules
}

// Site holds the rules of a domain together with its path overrides.
type Site struct {
	Rules Rules
	paths []pathRules // longest prefix first
}

// compileSite validates a domain configuration and merges each path override into it.
func compileSite(domain config.DomainConfig) (*Site, error) {
	base, err := compileRules(domain)
	if err != nil {
		return nil, err
	}

	site := &Site{Rules: base}
	for prefix, override := range domain.PathOverrides {
		if prefix == "" {
			return nil, errors.New("path override prefix must not be empty")
		}
		merged, err := compileRules(config.Merge(domain, override))
		if err != nil {
			return nil, fmt.Errorf("path override %q: %w", prefix, err)
		}
		site.paths = append(site.paths, pathRules{prefix: prefix, rules: merged})
	}
	sort.Slice(site.paths, func(i, j int) bool {
		return len(site.paths[i].prefix) > len(site.paths[j].prefix)
	})
	return site, nil
}

func compileRules(c config.DomainConfig) (Rules, error) {
	excluded, err := compilePatterns(c.ExcludedPaths)
	if err != nil {
		return Rules{}, fmt.Errorf("excludedPaths: %w", err)
	}
	included, err := compilePatterns(c.IncludedPaths)
	if err != nil {
		return Rules{}, fmt.Errorf("includedPaths: %w", err)
	}
	return Rules{
		TrackingEnabled:    c.TrackingEnabled,
		IdSite:             c.IdSite,
		trackedMethods:     c.TrackedMethods,
		excludedPaths:      excluded,
		includedPaths:      included,
		responseConditions: c.ResponseConditions,
	}, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// For returns the rules of the most specific path override matching path, or the domain
// rules if none matches. The second result is the matched prefix, empty for the domain rules.
func (s *Site) For(path string) (Rules, string) {
	for _, p := range s.paths {
		if pathMatchesPrefix(path, p.prefix) {
			return p.rules, p.prefix
		}
	}
	return s.Rules, ""
}

// TracksRequest reports whether the request qualifies for tracking before its response is known.
func (r Rules) TracksRequest(method, path string) bool {
	return r.TrackingEnabled &&
		isMethodTracked(method, r.trackedMethods) &&
		!isPathExcluded(path, r.excludedPaths, r.includedPaths)
}

// InspectsResponse reports whether the decision depends on the response status or headers.
func (r Rules) InspectsResponse() bool {
	rc := r.responseConditions
	return rc != nil && (len(rc.TrackOnStatusCodes) > 0 || len(rc.TrackWhenHeaders) > 0)
}

// TracksResponse reports whether the final response qualifies for tracking.
func (r Rules) TracksResponse(status int, hdr http.Header) bool {
	return matchesResponseConditions(status, hdr, r.responseConditions)
}

func pathMatchesPrefix(path, prefix string) bool {
	// Exact match or subpath with trailing slash
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// isPathExcluded reports whether path matches an excluded pattern and no included pattern.
func isPathExcluded(path string, excludedPaths, includedPaths []*regexp.Regexp) bool {
	if !matchesAny(path, excludedPaths) {
		return false
	}
	return !matchesAny(path, includedPaths)
}

func matchesAny(path string, patterns []*regexp.Regexp) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(path) {
			return true
		}
	}
	return false
}

func isMethodTracked(method string, trackedMethods []string) bool {
	// An omitted list preserves the original behavior of tracking every method.
	if trackedMethods == nil {
		return true
	}

	for _, trackedMethod := range trackedMethods {
		if strings.EqualFold(method, trackedMethod) {
			return true
		}
	}

	return false
}

// matchesResponseConditions returns true if rc is nil or all conditions match.
func matchesResponseConditions(status int, hdr http.Header, rc *config.ResponseConditions) bool {
	if rc == nil {
		return true
	}
	// Status filter
	if len(rc.TrackOnStatusCodes) > 0 {
		ok := false
		for _, s := range rc.TrackOnStatusCodes {
			if s == status {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	// Header filters (exact value match)
	for k, want := range rc.TrackWhenHeaders {
		found := false
		for _, v := range hdr.Values(k) {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
