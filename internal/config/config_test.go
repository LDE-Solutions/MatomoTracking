package config

import (
	"reflect"
	"testing"
)

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestMerge(t *testing.T) {
	t.Parallel()

	base := DomainConfig{
		TrackingEnabled: true,
		IdSite:          1,
		ExcludedPaths:   []string{"/admin"},
		IncludedPaths:   []string{`\.php$`},
	}

	override := PathConfig{
		TrackingEnabled: boolPtr(false),
		IdSite:          intPtr(42),
		ExcludedPaths:   []string{"/private"},
		IncludedPaths:   []string{`\.aspx$`},
	}

	got := Merge(base, override)

	if got.TrackingEnabled != false {
		t.Fatalf("TrackingEnabled = %v; want false", got.TrackingEnabled)
	}
	if got.IdSite != 42 {
		t.Fatalf("IdSite = %d; want 42", got.IdSite)
	}
	if len(got.ExcludedPaths) != 1 || got.ExcludedPaths[0] != "/private" {
		t.Fatalf("ExcludedPaths = %#v; want [/private]", got.ExcludedPaths)
	}
	if len(got.IncludedPaths) != 1 || got.IncludedPaths[0] != `\.aspx$` {
		t.Fatalf("IncludedPaths = %#v; want [\\.aspx$]", got.IncludedPaths)
	}
}

func TestMergeTrackedMethods(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     []string
		override []string
		want     []string
	}{
		{"omitted everywhere", nil, nil, nil},
		{"inherit domain list", []string{"GET"}, nil, []string{"GET"}},
		{"replace domain list", []string{"GET"}, []string{"POST"}, []string{"POST"}},
		{"empty override tracks none", []string{"GET"}, []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Merge(DomainConfig{TrackedMethods: tt.base}, PathConfig{TrackedMethods: tt.override})
			if !reflect.DeepEqual(got.TrackedMethods, tt.want) {
				t.Fatalf("TrackedMethods = %#v; want %#v", got.TrackedMethods, tt.want)
			}
		})
	}
}

func TestDefaultDomainUsesTopLevelRules(t *testing.T) {
	t.Parallel()

	config := &Config{
		TrackingEnabled:    true,
		IdSite:             7,
		TrackedMethods:     []string{"GET"},
		ExcludedPaths:      []string{`\.css$`},
		IncludedPaths:      []string{`\.php$`},
		PathOverrides:      map[string]PathConfig{"/api": {IdSite: intPtr(8)}},
		ResponseConditions: &ResponseConditions{TrackOnStatusCodes: []int{200}},
	}

	want := DomainConfig{
		TrackingEnabled:    true,
		IdSite:             7,
		TrackedMethods:     []string{"GET"},
		ExcludedPaths:      []string{`\.css$`},
		IncludedPaths:      []string{`\.php$`},
		PathOverrides:      map[string]PathConfig{"/api": {IdSite: intPtr(8)}},
		ResponseConditions: &ResponseConditions{TrackOnStatusCodes: []int{200}},
	}
	if got := config.DefaultDomain(); !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultDomain() = %#v; want %#v", got, want)
	}
}
