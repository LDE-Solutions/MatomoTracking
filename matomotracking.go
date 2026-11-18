// Package MatomoTracking is a Traefik middleware plugin that tracks requests server-side in Matomo.
//
// Traefik expects Config, CreateConfig and New in the root package of a plugin. The
// implementation lives in the internal packages.
package MatomoTracking

import (
	"context"
	"net/http"

	"github.com/DIE-Bonn/MatomoTracking/internal/config"
	"github.com/DIE-Bonn/MatomoTracking/internal/logger"
	"github.com/DIE-Bonn/MatomoTracking/internal/middleware"
)

// Configuration types of the plugin, see the config package for the fields.
type (
	Config             = config.Config
	DomainConfig       = config.DomainConfig
	PathConfig         = config.PathConfig
	ResponseConditions = config.ResponseConditions
)

// CreateConfig returns the default configuration for the plugin.
func CreateConfig() *Config {
	return &Config{}
}

// New creates a new instance of the MatomoTracking middleware.
//
// An invalid configuration disables tracking instead of failing: Traefik would otherwise
// disable every router using the middleware, taking the sites down over an analytics setting.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	handler, err := middleware.New(next, config, name)
	if err != nil {
		logger.New(name, false).Errorf("invalid configuration, tracking disabled: %v", err)
		return next, nil
	}
	return handler, nil
}
