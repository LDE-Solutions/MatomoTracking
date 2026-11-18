package MatomoTracking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPassesThroughOnInvalidConfig(t *testing.T) {
	t.Parallel()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	handler, err := New(context.Background(), next, CreateConfig(), "test") // matomoURL missing
	if err != nil {
		t.Fatalf("New() error = %v; want pass-through handler", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://demo.localhost/", nil))
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d; want request forwarded to next", recorder.Code)
	}
}

func TestNewReturnsMiddleware(t *testing.T) {
	t.Parallel()

	config := CreateConfig()
	config.MatomoURL = "https://matomo.example.com/matomo.php"
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	handler, err := New(context.Background(), next, config, "test")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, ok := handler.(http.HandlerFunc); ok {
		t.Fatal("New() returned next; want the tracking middleware")
	}
}
