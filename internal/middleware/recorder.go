package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// statusRecorder captures the final status while delegating to the real ResponseWriter.
// It passes flushing and hijacking through. Under Traefik's Yaegi interpreter only Hijack
// reaches Traefik (Yaegi has no wrapper for Flusher), so the middleware uses the recorder only
// when response conditions need the status.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	// Default to 200 unless WriteHeader is called explicitly.
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (w *statusRecorder) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush sends buffered data to the client.
func (w *statusRecorder) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack hands the connection over to the caller, e.g. for a WebSocket upgrade. The response
// is then written to the connection directly, so the status is recorded as 101.
func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("%T does not support hijacking", w.ResponseWriter)
	}
	conn, rw, err := hijacker.Hijack()
	if err == nil {
		w.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

// Unwrap lets http.ResponseController reach the underlying ResponseWriter.
func (w *statusRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
