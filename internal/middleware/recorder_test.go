package middleware

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// hijackableWriter is a ResponseWriter that supports hijacking, like the one of net/http.
type hijackableWriter struct {
	*httptest.ResponseRecorder
	conn net.Conn
}

func (w *hijackableWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(bufio.NewReader(w.conn), bufio.NewWriter(w.conn)), nil
}

func TestStatusRecorderFlush(t *testing.T) {
	t.Parallel()

	underlying := httptest.NewRecorder()
	rec := newStatusRecorder(underlying)
	http.NewResponseController(rec).Flush()
	if !underlying.Flushed {
		t.Fatal("Flush was not passed through")
	}
}

func TestStatusRecorderHijack(t *testing.T) {
	t.Parallel()

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	rec := newStatusRecorder(&hijackableWriter{ResponseRecorder: httptest.NewRecorder(), conn: server})
	conn, _, err := http.NewResponseController(rec).Hijack()
	if err != nil {
		t.Fatalf("Hijack() error = %v", err)
	}
	if conn != server {
		t.Fatal("Hijack() returned a different connection")
	}
	if rec.status != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d; want %d", rec.status, http.StatusSwitchingProtocols)
	}
}

func TestStatusRecorderHijackUnsupported(t *testing.T) {
	t.Parallel()

	rec := newStatusRecorder(httptest.NewRecorder())
	if _, _, err := rec.Hijack(); err == nil {
		t.Fatal("Hijack() succeeded; want error")
	}
	if rec.status != http.StatusOK {
		t.Fatalf("status = %d; want unchanged %d", rec.status, http.StatusOK)
	}
}

func TestStatusRecorderUnwrap(t *testing.T) {
	t.Parallel()

	underlying := httptest.NewRecorder()
	rec := newStatusRecorder(underlying)
	if rec.Unwrap() != underlying {
		t.Fatal("Unwrap() did not return the underlying writer")
	}
}
