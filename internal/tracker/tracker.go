// Package tracker sends page views to the Matomo tracking API.
package tracker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DIE-Bonn/MatomoTracking/internal/logger"
)

// PageView is the request data a Matomo tracking call needs. It is captured before the request
// is forwarded, so handlers further down the chain cannot change what gets tracked.
type PageView struct {
	url          string
	userAgent    string
	forwardedFor string
}

// NewPageView captures the tracking data of req, reported under host.
func NewPageView(req *http.Request, host string) (PageView, error) {
	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return PageView{}, fmt.Errorf("reading client IP: %w", err)
	}

	uri, err := url.Parse(req.URL.RequestURI())
	if err != nil {
		return PageView{}, fmt.Errorf("parsing request URI: %w", err)
	}
	uri.Path = strings.ToLower(uri.Path)

	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	// Append the client IP to keep the X-Forwarded-For chain. The first entry is the original
	// client, so Matomo should be configured to read the first entry.
	forwardedFor := clientIP
	if existing := req.Header.Get("X-Forwarded-For"); existing != "" {
		forwardedFor = existing + "," + clientIP
	}

	return PageView{
		url:          scheme + "://" + host + uri.String(),
		userAgent:    req.Header.Get("User-Agent"),
		forwardedFor: forwardedFor,
	}, nil
}

// Tracker sends page views to the Matomo tracking API.
type Tracker struct {
	endpoint *url.URL
	client   *http.Client
	log      *logger.Logger
}

// New validates matomoURL and returns a tracker for it.
func New(matomoURL string, log *logger.Logger) (*Tracker, error) {
	if matomoURL == "" {
		return nil, errors.New("matomoURL is required")
	}
	endpoint, err := url.Parse(matomoURL)
	if err != nil {
		return nil, fmt.Errorf("matomoURL: %w", err)
	}
	if endpoint.Scheme == "" || endpoint.Host == "" {
		return nil, fmt.Errorf("matomoURL %q must be an absolute URL", matomoURL)
	}

	return &Tracker{
		endpoint: endpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
		log:      log,
	}, nil
}

// trackingURL returns the Matomo endpoint with the tracking parameters for view.
func (t *Tracker) trackingURL(view PageView, idSite int) string {
	u := *t.endpoint
	query := u.Query()
	query.Set("url", view.url)
	query.Set("rec", "1")
	query.Set("idsite", strconv.Itoa(idSite))
	u.RawQuery = query.Encode()
	return u.String()
}

// Track sends view to Matomo. It blocks until Matomo answers and is meant to run in its own goroutine.
func (t *Tracker) Track(view PageView, idSite int) {
	req, err := http.NewRequest(http.MethodGet, t.trackingURL(view, idSite), nil)
	if err != nil {
		t.log.Errorf("creating Matomo request: %v", err)
		return
	}
	req.Header.Set("User-Agent", view.userAgent)
	req.Header.Set("X-Forwarded-For", view.forwardedFor)

	resp, err := t.client.Do(req)
	if err != nil {
		// *url.Error repeats the full Matomo URL, which carries the page URL and possibly a token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		t.log.Errorf("sending Matomo request to %s for %s: %v", redactURL(t.endpoint.String()), redactURL(view.url), err)
		return
	}
	defer resp.Body.Close()
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusBadRequest {
		t.log.Errorf("Matomo answered %s for %s (idSite %d)", resp.Status, redactURL(view.url), idSite)
		return
	}
	t.log.Debugf("tracked %s (idSite %d): %s", view.url, idSite, resp.Status)
}

// redactURL strips credentials, query and fragment from rawURL for error logs, as they may
// contain passwords, personal data or tokens. Debug logs keep the full URL.
func redactURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid URL>"
	}
	u.User = nil
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}
