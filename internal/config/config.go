// Package config defines the plugin configuration as Traefik passes it to the middleware.
package config

// Config represents the configuration for the MatomoTracking plugin.
//
// Traefik fills it from the dynamic configuration, e.g. a file provider or Docker labels.
// Traefik matches keys case-insensitively against the field names; the json tags are only
// used when the configuration is decoded directly (e.g. in tests).
type Config struct {
	// MatomoURL is the Matomo tracking endpoint, e.g. https://matomo.example.com/matomo.php.
	MatomoURL string `json:"matomoURL,omitempty"`
	// Debug logs every tracking decision.
	Debug bool `json:"debug,omitempty"`
	// Domains holds the rules per request host. Use it when one middleware serves several hosts,
	// e.g. when it is attached to an entry point.
	Domains map[string]DomainConfig `json:"domains,omitempty"`

	// The following fields are the rules for every host without an entry in Domains. They sit at
	// the top level so a middleware attached to a single router, e.g. through Docker labels,
	// needs no host map (host names contain dots, which label keys cannot express).
	TrackingEnabled    bool                  `json:"trackingEnabled,omitempty"`
	IdSite             int                   `json:"idSite,omitempty"`
	TrackedMethods     []string              `json:"trackedMethods,omitempty"`
	ExcludedPaths      []string              `json:"excludedPaths,omitempty"`
	IncludedPaths      []string              `json:"includedPaths,omitempty"`
	PathOverrides      map[string]PathConfig `json:"pathOverrides,omitempty"`
	ResponseConditions *ResponseConditions   `json:"responseConditions,omitempty"`
}

// DomainConfig specifies the tracking rules for a specific domain.
type DomainConfig struct {
	TrackingEnabled    bool                  `json:"trackingEnabled,omitempty"`
	IdSite             int                   `json:"idSite,omitempty"`
	TrackedMethods     []string              `json:"trackedMethods,omitempty"`
	ExcludedPaths      []string              `json:"excludedPaths,omitempty"`
	IncludedPaths      []string              `json:"includedPaths,omitempty"`
	PathOverrides      map[string]PathConfig `json:"pathOverrides,omitempty"`
	ResponseConditions *ResponseConditions   `json:"responseConditions,omitempty"`
}

// PathConfig specifies the tracking rules for a specific path. Unset fields inherit the
// value of the domain.
type PathConfig struct {
	TrackingEnabled    *bool               `json:"trackingEnabled,omitempty"`
	IdSite             *int                `json:"idSite,omitempty"`
	TrackedMethods     []string            `json:"trackedMethods,omitempty"`
	ExcludedPaths      []string            `json:"excludedPaths,omitempty"`
	IncludedPaths      []string            `json:"includedPaths,omitempty"`
	ResponseConditions *ResponseConditions `json:"responseConditions,omitempty"`
}

// ResponseConditions define when to track based on the final response.
type ResponseConditions struct {
	// Track only if the final status code is one of these. Empty = allow any.
	TrackOnStatusCodes []int `json:"trackOnStatusCodes,omitempty"`
	// All headers must be present and equal to the given value (exact match, case-insensitive keys).
	TrackWhenHeaders map[string]string `json:"trackWhenHeaders,omitempty"`
}

// DefaultDomain returns the top-level rules as a domain configuration.
func (c *Config) DefaultDomain() DomainConfig {
	return DomainConfig{
		TrackingEnabled:    c.TrackingEnabled,
		IdSite:             c.IdSite,
		TrackedMethods:     c.TrackedMethods,
		ExcludedPaths:      c.ExcludedPaths,
		IncludedPaths:      c.IncludedPaths,
		PathOverrides:      c.PathOverrides,
		ResponseConditions: c.ResponseConditions,
	}
}

// Merge applies a path override to the domain configuration. Every field set in the
// override replaces the domain value; slices are replaced, not appended.
func Merge(base DomainConfig, override PathConfig) DomainConfig {
	merged := base

	if override.TrackingEnabled != nil {
		merged.TrackingEnabled = *override.TrackingEnabled
	}
	if override.IdSite != nil {
		merged.IdSite = *override.IdSite
	}
	if override.TrackedMethods != nil {
		merged.TrackedMethods = override.TrackedMethods
	}
	if override.ExcludedPaths != nil {
		merged.ExcludedPaths = override.ExcludedPaths
	}
	if override.IncludedPaths != nil {
		merged.IncludedPaths = override.IncludedPaths
	}
	if override.ResponseConditions != nil {
		merged.ResponseConditions = override.ResponseConditions
	}
	return merged
}
