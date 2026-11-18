package tracker

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DIE-Bonn/MatomoTracking/internal/logger"
)

func TestNewPageView(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		target           string
		tls              bool
		forwardedFor     string
		wantURL          string
		wantForwardedFor string
	}{
		{"lower-cases path", "http://demo.localhost/Some/Page?Q=Keep", false, "", "http://demo.localhost/some/page?Q=Keep", "203.0.113.9"},
		{"https scheme", "https://demo.localhost/", true, "", "https://demo.localhost/", "203.0.113.9"},
		{"appends to forwarded chain", "http://demo.localhost/", false, "198.51.100.1", "http://demo.localhost/", "198.51.100.1,203.0.113.9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.RemoteAddr = "203.0.113.9:54321"
			req.Header.Set("User-Agent", "Test-UA")
			if tt.forwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.forwardedFor)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			view, err := NewPageView(req, "demo.localhost")
			if err != nil {
				t.Fatal(err)
			}
			if view.url != tt.wantURL {
				t.Fatalf("url = %q; want %q", view.url, tt.wantURL)
			}
			if view.forwardedFor != tt.wantForwardedFor {
				t.Fatalf("forwardedFor = %q; want %q", view.forwardedFor, tt.wantForwardedFor)
			}
			if view.userAgent != "Test-UA" {
				t.Fatalf("userAgent = %q; want Test-UA", view.userAgent)
			}
		})
	}
}

func TestNewPageViewRejectsInvalidRemoteAddr(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "http://demo.localhost/", nil)
	req.RemoteAddr = "not-an-address"
	if _, err := NewPageView(req, "demo.localhost"); err == nil {
		t.Fatal("NewPageView() succeeded; want error")
	}
}

func TestNewValidatesURL(t *testing.T) {
	t.Parallel()

	for _, matomoURL := range []string{"", "matomo.php", "/matomo.php", "http://[::1"} {
		if _, err := New(matomoURL, logger.New("test", false)); err == nil {
			t.Fatalf("New(%q) succeeded; want error", matomoURL)
		}
	}
}

func TestTrackingURLKeepsEndpointQuery(t *testing.T) {
	t.Parallel()

	tr, err := New("https://matomo.example.com/matomo.php?token=abc", logger.New("test", false))
	if err != nil {
		t.Fatal(err)
	}

	got, err := url.Parse(tr.trackingURL(PageView{url: "http://demo.localhost/a?b=c"}, 5))
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "matomo.example.com" || got.Path != "/matomo.php" {
		t.Fatalf("endpoint = %s%s; want matomo.example.com/matomo.php", got.Host, got.Path)
	}
	want := url.Values{"token": {"abc"}, "url": {"http://demo.localhost/a?b=c"}, "rec": {"1"}, "idsite": {"5"}}
	if q := got.Query(); q.Encode() != want.Encode() {
		t.Fatalf("query = %v; want %v", q, want)
	}
}

func TestTrackErrorLogsOmitQueries(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	log := logger.New("test", false)
	log.SetOutput(&out)

	// Nothing listens on port 1, so the request fails with a *url.Error.
	tr, err := New("http://user:password@127.0.0.1:1/matomo.php?token_auth=secret", log)
	if err != nil {
		t.Fatal(err)
	}
	tr.Track(PageView{url: "http://demo.localhost/reset?token=private"}, 1)

	logged := out.String()
	if !strings.Contains(logged, "ERROR") {
		t.Fatalf("expected an error log, got %q", logged)
	}
	for _, leak := range []string{"user", "password", "secret", "private", "token"} {
		if strings.Contains(logged, leak) {
			t.Fatalf("error log contains %q: %s", leak, logged)
		}
	}
}

func TestRedactURL(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"http://a/b?c=d":                 "http://a/b",
		"http://a/b#frag":                "http://a/b",
		"http://a/b":                     "http://a/b",
		"http://a/b?c#d=e":               "http://a/b",
		"http://a/b?":                    "http://a/b",
		"https://user:pass@a:8443/b?c=d": "https://a:8443/b",
		"https://user@a/b":               "https://a/b",
		"http://[::1":                    "<invalid URL>",
	}
	for in, want := range tests {
		if got := redactURL(in); got != want {
			t.Fatalf("redactURL(%q) = %q; want %q", in, got, want)
		}
	}
}
