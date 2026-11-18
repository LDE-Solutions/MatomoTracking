// Package middleware forwards requests and tracks them in Matomo according to the rules.
package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/DIE-Bonn/MatomoTracking/internal/config"
	"github.com/DIE-Bonn/MatomoTracking/internal/logger"
	"github.com/DIE-Bonn/MatomoTracking/internal/rules"
	"github.com/DIE-Bonn/MatomoTracking/internal/tracker"
)

// Middleware is the http.Handler Traefik puts into the chain.
type Middleware struct {
	next    http.Handler
	sites   *rules.Sites
	tracker *tracker.Tracker
	log     *logger.Logger
}

// New validates cfg and returns the middleware. Configuration mistakes such as invalid
// patterns surface when Traefik loads the middleware instead of on each request.
func New(next http.Handler, cfg *config.Config, name string) (*Middleware, error) {
	log := logger.New(name, cfg.Debug)

	t, err := tracker.New(cfg.MatomoURL, log)
	if err != nil {
		return nil, err
	}

	sites, err := rules.Compile(cfg)
	if err != nil {
		return nil, err
	}

	return &Middleware{next: next, sites: sites, tracker: t, log: log}, nil
}

// ServeHTTP forwards every request unchanged and tracks it in Matomo afterwards if the
// rules for its host and path allow it.
func (m *Middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	host := requestHost(req)
	site := m.sites.For(host)

	// A disabled domain stays disabled, even if a path override enables tracking.
	if !site.Rules.TrackingEnabled {
		m.log.Debugf("%s %s%s: tracking disabled for host", req.Method, host, req.URL.Path)
		m.next.ServeHTTP(rw, req)
		return
	}

	r, prefix := site.For(req.URL.Path)
	if !r.TracksRequest(req.Method, req.URL.Path) {
		m.log.Debugf("%s %s%s: skipped (override %q): disabled, method not tracked or path excluded", req.Method, host, req.URL.Path, prefix)
		m.next.ServeHTTP(rw, req)
		return
	}

	view, err := tracker.NewPageView(req, host)
	if err != nil {
		m.log.Errorf("%s %s%s: %v", req.Method, host, req.URL.Path, err)
		m.next.ServeHTTP(rw, req)
		return
	}

	rec := newStatusRecorder(rw)
	m.next.ServeHTTP(rec, req)

	if !r.TracksResponse(rec.status, rec.Header()) {
		m.log.Debugf("%s %s%s: skipped: response conditions not met (status %d)", req.Method, host, req.URL.Path, rec.status)
		return
	}

	m.log.Debugf("%s %s%s: tracking (override %q, idSite %d)", req.Method, host, req.URL.Path, prefix, r.IdSite)
	go m.tracker.Track(view, r.IdSite)
}

// requestHost returns the lower-case request host without port.
func requestHost(req *http.Request) string {
	host := req.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}
