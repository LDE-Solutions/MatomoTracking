package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/DIE-Bonn/MatomoTracking/internal/config"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

// serve runs one request through the middleware and returns the query of the Matomo tracking
// call, or nil if none arrived. cfg.MatomoURL is replaced by a test server.
func serve(t *testing.T, cfg *config.Config, method, target string, wantTracking bool) url.Values {
	t.Helper()

	trackingCalls := make(chan *http.Request, 1)
	matomo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trackingCalls <- r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer matomo.Close()
	cfg.MatomoURL = matomo.URL

	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = "203.0.113.9:54321"
	forwarded := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded = true
		if r != req || r.Method != method {
			t.Fatal("original request or method changed before forwarding")
		}
		// Handlers further down may rewrite the path; tracking must use the original one.
		r.URL.Path = "/rewritten"
		w.WriteHeader(http.StatusAccepted)
	})
	handler, err := New(next, cfg, "test")
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if !forwarded || recorder.Code != http.StatusAccepted {
		t.Fatalf("request forwarding failed: forwarded=%v, status=%d", forwarded, recorder.Code)
	}

	timeout := 200 * time.Millisecond
	if wantTracking {
		timeout = 2 * time.Second
	}
	select {
	case r := <-trackingCalls:
		if !wantTracking {
			t.Fatalf("unexpected Matomo tracking request: %s", r.URL)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("Matomo request method = %q; want GET", r.Method)
		}
		return r.URL.Query()
	case <-time.After(timeout):
		if wantTracking {
			t.Fatal("expected Matomo tracking request was not received")
		}
		return nil
	}
}

func TestServeHTTPTrackedMethods(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		trackedMethods []string
		override       *config.PathConfig
		wantTracking   bool
	}{
		{"omitted tracks HEAD", http.MethodHead, nil, nil, true},
		{"omitted tracks OPTIONS", http.MethodOptions, nil, nil, true},
		{"GET list tracks GET", http.MethodGet, []string{"GET"}, nil, true},
		{"GET list skips HEAD", http.MethodHead, []string{"GET"}, nil, false},
		{"GET list skips OPTIONS", http.MethodOptions, []string{"GET"}, nil, false},
		{"explicit POST", http.MethodPost, []string{"GET", "POST"}, nil, true},
		{"empty list skips GET", http.MethodGet, []string{}, nil, false},
		{"path inherits domain list", http.MethodHead, []string{"GET"}, &config.PathConfig{}, false},
		{"path replaces domain list", http.MethodPost, []string{"GET"}, &config.PathConfig{TrackedMethods: []string{"POST"}}, true},
		{"path replacement excludes GET", http.MethodGet, []string{"GET"}, &config.PathConfig{TrackedMethods: []string{"POST"}}, false},
		{"empty path override skips GET", http.MethodGet, []string{"GET"}, &config.PathConfig{TrackedMethods: []string{}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain := config.DomainConfig{TrackingEnabled: true, IdSite: 1, TrackedMethods: tt.trackedMethods}
			if tt.override != nil {
				domain.PathOverrides = map[string]config.PathConfig{"/api": *tt.override}
			}
			cfg := &config.Config{Domains: map[string]config.DomainConfig{"demo.localhost": domain}}
			serve(t, cfg, tt.method, "http://demo.localhost/api/test", tt.wantTracking)
		})
	}
}

func TestServeHTTPSiteSelection(t *testing.T) {
	tests := []struct {
		name       string
		config     *config.Config
		target     string
		wantIdSite string // empty: no tracking
	}{
		{
			name:   "unknown host without top-level rules",
			config: &config.Config{Domains: map[string]config.DomainConfig{"demo.localhost": {TrackingEnabled: true, IdSite: 1}}},
			target: "http://other.localhost/",
		},
		{
			name:       "top-level rules without domains",
			config:     &config.Config{TrackingEnabled: true, IdSite: 5},
			target:     "http://labels.localhost/page",
			wantIdSite: "5",
		},
		{
			name: "top-level rules for unknown host",
			config: &config.Config{
				TrackingEnabled: true,
				IdSite:          5,
				Domains:         map[string]config.DomainConfig{"demo.localhost": {TrackingEnabled: true, IdSite: 1}},
			},
			target:     "http://other.localhost/",
			wantIdSite: "5",
		},
		{
			name: "domain entry wins over top-level rules",
			config: &config.Config{
				TrackingEnabled: true,
				IdSite:          5,
				Domains:         map[string]config.DomainConfig{"demo.localhost": {TrackingEnabled: true, IdSite: 1}},
			},
			target:     "http://demo.localhost/",
			wantIdSite: "1",
		},
		{
			name: "disabled domain entry wins over top-level rules",
			config: &config.Config{
				TrackingEnabled: true,
				IdSite:          5,
				Domains:         map[string]config.DomainConfig{"demo.localhost": {TrackingEnabled: false, IdSite: 1}},
			},
			target: "http://demo.localhost/",
		},
		{
			name:       "host matches case-insensitively and without port",
			config:     &config.Config{Domains: map[string]config.DomainConfig{"Demo.Localhost": {TrackingEnabled: true, IdSite: 1}}},
			target:     "http://DEMO.localhost:8080/",
			wantIdSite: "1",
		},
		{
			name: "path override cannot enable a disabled domain",
			config: &config.Config{Domains: map[string]config.DomainConfig{"demo.localhost": {
				IdSite:        1,
				PathOverrides: map[string]config.PathConfig{"/api": {TrackingEnabled: boolPtr(true)}},
			}}},
			target: "http://demo.localhost/api",
		},
		{
			name: "top-level path override",
			config: &config.Config{
				TrackingEnabled: true,
				IdSite:          5,
				PathOverrides:   map[string]config.PathConfig{"/shop": {IdSite: intPtr(6)}},
			},
			target:     "http://labels.localhost/shop/cart",
			wantIdSite: "6",
		},
		{
			name: "response conditions not met",
			config: &config.Config{
				TrackingEnabled:    true,
				IdSite:             5,
				ResponseConditions: &config.ResponseConditions{TrackOnStatusCodes: []int{200}},
			},
			target: "http://labels.localhost/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := serve(t, tt.config, http.MethodGet, tt.target, tt.wantIdSite != "")
			if tt.wantIdSite == "" {
				return
			}
			if got := query.Get("idsite"); got != tt.wantIdSite {
				t.Fatalf("idsite = %q; want %q", got, tt.wantIdSite)
			}
		})
	}
}

func TestServeHTTPTracksOriginalURL(t *testing.T) {
	cfg := &config.Config{TrackingEnabled: true, IdSite: 1}
	query := serve(t, cfg, http.MethodGet, "http://Demo.Localhost:8080/Some/Page?x=1", true)
	if got, want := query.Get("url"), "http://demo.localhost/some/page?x=1"; got != want {
		t.Fatalf("url = %q; want %q", got, want)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config *config.Config
	}{
		{"missing matomoURL", &config.Config{}},
		{"relative matomoURL", &config.Config{MatomoURL: "/matomo.php"}},
		{"invalid domain pattern", &config.Config{
			MatomoURL: "http://matomo/matomo.php",
			Domains:   map[string]config.DomainConfig{"demo.localhost": {ExcludedPaths: []string{"("}}},
		}},
		{"invalid top-level pattern", &config.Config{MatomoURL: "http://matomo/matomo.php", IncludedPaths: []string{"("}}},
	}

	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(next, tt.config, "test"); err == nil {
				t.Fatal("New() succeeded; want error")
			}
		})
	}
}

func TestServeHTTPWrapsWriterOnlyForResponseConditions(t *testing.T) {
	tests := []struct {
		name     string
		rc       *config.ResponseConditions
		wantWrap bool
	}{
		{"no conditions", nil, false},
		{"empty conditions", &config.ResponseConditions{}, false},
		{"status conditions", &config.ResponseConditions{TrackOnStatusCodes: []int{200}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{MatomoURL: "http://127.0.0.1:1/matomo.php", TrackingEnabled: true, IdSite: 1, ResponseConditions: tt.rc}
			recorder := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, wrapped := w.(*statusRecorder); wrapped != tt.wantWrap {
					t.Fatalf("writer wrapped = %v; want %v", wrapped, tt.wantWrap)
				}
			})
			handler, err := New(next, cfg, "test")
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "http://demo.localhost/", nil)
			req.RemoteAddr = "203.0.113.9:54321"
			handler.ServeHTTP(recorder, req)
		})
	}
}
