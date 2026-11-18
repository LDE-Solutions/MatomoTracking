package rules

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/DIE-Bonn/MatomoTracking/internal/config"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func mustCompile(patterns ...string) []*regexp.Regexp {
	compiled, err := compilePatterns(patterns)
	if err != nil {
		panic(err)
	}
	return compiled
}

func TestPathMatchesPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		{"/test", "/test", true},
		{"/test/sub", "/test", true},
		{"/test2", "/test", false},
		{"/testing", "/test", false},
		{"/", "/", true},
		{"/a/b", "/a", true},
		{"/a", "/a/b", false},
	}

	for _, tt := range tests {
		if got := pathMatchesPrefix(tt.path, tt.prefix); got != tt.want {
			t.Fatalf("pathMatchesPrefix(%q, %q) = %v; want %v", tt.path, tt.prefix, got, tt.want)
		}
	}
}

func TestIsPathExcluded(t *testing.T) {
	t.Parallel()

	// Matches files with extensions (e.g., .css, .png) optionally followed by query
	excluded := mustCompile(`\.\w{1,5}(\?.+)?$`)
	// Includes PHP and ASPX explicitly even if excluded matched
	included := mustCompile(`\.php(\?.*)?$`, `\.aspx(\?.*)?$`)

	cases := []struct {
		path string
		want bool
	}{
		{"/index.php", false},      // included explicitly
		{"/INDEX.PHP", true},       // included regex patterns are case-sensitive -> not included, so excluded by ext
		{"/api/data", false},       // no excluded match
		{"/image.png", true},       // excluded by ext
		{"/style.CSS", true},       // excluded by ext
		{"/page.aspx?x=1", false},  // included explicitly
		{"/download.tar.gz", true}, // .gz matches (3 letters)
		{"/noext", false},          // no excluded match
	}

	for _, tc := range cases {
		got := isPathExcluded(tc.path, excluded, included)
		if got != tc.want {
			t.Fatalf("isPathExcluded(%q) = %v; want %v", tc.path, got, tc.want)
		}
	}
}

func TestTrackedMethodsConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config string
		method string
		want   bool
	}{
		{"omitted allows GET", `{}`, http.MethodGet, true},
		{"omitted allows HEAD", `{}`, http.MethodHead, true},
		{"omitted allows OPTIONS", `{}`, http.MethodOptions, true},
		{"listed GET", `{"trackedMethods":["GET"]}`, http.MethodGet, true},
		{"unlisted HEAD", `{"trackedMethods":["GET"]}`, http.MethodHead, false},
		{"unlisted OPTIONS", `{"trackedMethods":["GET"]}`, http.MethodOptions, false},
		{"multiple methods", `{"trackedMethods":["GET","POST"]}`, http.MethodPost, true},
		{"case insensitive", `{"trackedMethods":["post"]}`, http.MethodPost, true},
		{"empty allows none", `{"trackedMethods":[]}`, http.MethodGet, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var domain config.DomainConfig
			if err := json.Unmarshal([]byte(tt.config), &domain); err != nil {
				t.Fatal(err)
			}
			if got := isMethodTracked(tt.method, domain.TrackedMethods); got != tt.want {
				t.Fatalf("isMethodTracked(%q, %#v) = %v; want %v", tt.method, domain.TrackedMethods, got, tt.want)
			}
		})
	}
}

func TestCompileSiteRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		domain config.DomainConfig
	}{
		{"invalid excluded pattern", config.DomainConfig{ExcludedPaths: []string{"("}}},
		{"invalid included pattern", config.DomainConfig{IncludedPaths: []string{"["}}},
		{"invalid override pattern", config.DomainConfig{PathOverrides: map[string]config.PathConfig{"/api": {ExcludedPaths: []string{"("}}}}},
		{"empty override prefix", config.DomainConfig{PathOverrides: map[string]config.PathConfig{"": {}}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := compileSite(tt.domain); err == nil {
				t.Fatal("compileSite() succeeded; want error")
			}
		})
	}
}

func TestSiteForPicksLongestPrefix(t *testing.T) {
	t.Parallel()

	site, err := compileSite(config.DomainConfig{
		TrackingEnabled: true,
		IdSite:          1,
		PathOverrides: map[string]config.PathConfig{
			"/a":     {IdSite: intPtr(2)},
			"/a/b":   {IdSite: intPtr(3)},
			"/a/b/c": {TrackingEnabled: boolPtr(false)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path       string
		wantPrefix string
		wantIdSite int
		wantOn     bool
	}{
		{"/", "", 1, true},
		{"/ab", "", 1, true},
		{"/a", "/a", 2, true},
		{"/a/x", "/a", 2, true},
		{"/a/b/x", "/a/b", 3, true},
		{"/a/b/c/d", "/a/b/c", 1, false}, // overrides merge onto the domain, not onto each other
	}

	for _, tt := range tests {
		got, prefix := site.For(tt.path)
		if prefix != tt.wantPrefix || got.IdSite != tt.wantIdSite || got.TrackingEnabled != tt.wantOn {
			t.Fatalf("For(%q) = prefix %q, idSite %d, enabled %v; want %q, %d, %v",
				tt.path, prefix, got.IdSite, got.TrackingEnabled, tt.wantPrefix, tt.wantIdSite, tt.wantOn)
		}
	}
}

func TestSitesFor(t *testing.T) {
	t.Parallel()

	sites, err := Compile(&config.Config{
		TrackingEnabled: true,
		IdSite:          9,
		Domains: map[string]config.DomainConfig{
			"Demo.Localhost": {TrackingEnabled: true, IdSite: 1},
			"off.localhost":  {IdSite: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		host       string
		wantIdSite int
		wantOn     bool
	}{
		{"demo.localhost", 1, true},
		{"DEMO.LOCALHOST", 1, true},
		{"off.localhost", 2, false}, // an explicit entry wins over the fallback
		{"other.localhost", 9, true},
	}

	for _, tt := range tests {
		got := sites.For(tt.host).Rules
		if got.IdSite != tt.wantIdSite || got.TrackingEnabled != tt.wantOn {
			t.Fatalf("For(%q) = idSite %d, enabled %v; want %d, %v", tt.host, got.IdSite, got.TrackingEnabled, tt.wantIdSite, tt.wantOn)
		}
	}
}

func TestCompileRejectsInvalidDomain(t *testing.T) {
	t.Parallel()

	tests := []*config.Config{
		{Domains: map[string]config.DomainConfig{"demo.localhost": {ExcludedPaths: []string{"("}}}},
		{IncludedPaths: []string{"("}},
	}
	for _, cfg := range tests {
		if _, err := Compile(cfg); err == nil {
			t.Fatalf("Compile(%#v) succeeded; want error", cfg)
		}
	}
}

func TestInspectsResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rc   *config.ResponseConditions
		want bool
	}{
		{"no conditions", nil, false},
		{"empty conditions", &config.ResponseConditions{}, false},
		{"status codes", &config.ResponseConditions{TrackOnStatusCodes: []int{200}}, true},
		{"headers", &config.ResponseConditions{TrackWhenHeaders: map[string]string{"X-App": "web"}}, true},
	}

	for _, tt := range tests {
		if got := (Rules{responseConditions: tt.rc}).InspectsResponse(); got != tt.want {
			t.Fatalf("%s: InspectsResponse() = %v; want %v", tt.name, got, tt.want)
		}
	}
}

func TestMatchesResponseConditions_Nil(t *testing.T) {
	t.Parallel()
	h := http.Header{}
	if !matchesResponseConditions(404, h, nil) {
		t.Fatalf("nil conditions should allow any status/headers")
	}
}

func TestMatchesResponseConditions_StatusOnly(t *testing.T) {
	t.Parallel()
	rc := &config.ResponseConditions{TrackOnStatusCodes: []int{200, 201}}
	h := http.Header{}

	if !matchesResponseConditions(200, h, rc) {
		t.Fatalf("expected status 200 to match")
	}
	if matchesResponseConditions(404, h, rc) {
		t.Fatalf("expected status 404 to NOT match")
	}
}

func TestMatchesResponseConditions_HeadersOnly(t *testing.T) {
	t.Parallel()

	// Exact value required; header name case-insensitive
	rc := &config.ResponseConditions{
		TrackWhenHeaders: map[string]string{
			"Content-Type": "text/html; charset=UTF-8",
		},
	}

	// Match exact value
	h := http.Header{}
	h.Set("content-type", "text/html; charset=UTF-8") // mixed case set
	if !matchesResponseConditions(200, h, rc) {
		t.Fatalf("expected header to match")
	}

	// Non-matching value
	h2 := http.Header{}
	h2.Set("Content-Type", "application/json")
	if matchesResponseConditions(200, h2, rc) {
		t.Fatalf("expected header to NOT match")
	}

	// Multiple values where one matches
	h3 := http.Header{}
	h3.Add("Content-Type", "application/json")
	h3.Add("Content-Type", "text/html; charset=UTF-8")
	if !matchesResponseConditions(200, h3, rc) {
		t.Fatalf("expected one-of multiple header values to match")
	}
}

func TestMatchesResponseConditions_BothStatusAndHeaders(t *testing.T) {
	t.Parallel()

	rc := &config.ResponseConditions{
		TrackOnStatusCodes: []int{200},
		TrackWhenHeaders: map[string]string{
			"X-App":        "web",
			"Content-Type": "text/html",
		},
	}

	// All match
	h := http.Header{}
	h.Set("X-App", "web")
	h.Set("Content-Type", "text/html")
	if !matchesResponseConditions(200, h, rc) {
		t.Fatalf("expected combined conditions to match")
	}

	// Status mismatch
	if matchesResponseConditions(404, h, rc) {
		t.Fatalf("expected status mismatch to fail")
	}

	// Header missing
	hMissing := http.Header{}
	hMissing.Set("X-App", "web")
	if matchesResponseConditions(200, hMissing, rc) {
		t.Fatalf("expected missing header to fail")
	}

	// Header present but value different
	hDiff := http.Header{}
	hDiff.Set("X-App", "api")
	hDiff.Set("Content-Type", "text/html")
	if matchesResponseConditions(200, hDiff, rc) {
		t.Fatalf("expected header value mismatch to fail")
	}
}
