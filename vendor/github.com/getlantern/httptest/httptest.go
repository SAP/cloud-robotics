// Package httptest provides a hijackable response recorder similar to the one
// in net/http/httptest but that also supports hijacking.
package httptest

import (
	"bufio"
	"bytes"
	"github.com/getlantern/mockconn"
	"net"
	"net/http"
	"net/http/httptest"
)

// Creates a new HijackableResponseRecorder that sends the given
// hijackedInputData (okay to leave nil).
func NewRecorder(hijackedInputData []byte) *HijackableResponseRecorder {
	wrapped := httptest.NewRecorder()
	h := &HijackableResponseRecorder{
		wrapped: wrapped,
		in:      bufio.NewReader(bytes.NewBuffer(hijackedInputData)),
		out:     bufio.NewWriter(wrapped.Body),
	}
	h.conn = mockconn.New(h.wrapped.Body, h.in)
	return h
}

// HijackableResponseRecorder is very similar to
// net/http/httputil.ResponseRecorder but also allows hijacking. The results of
// data received after hijacking are available in the Body() just as
// non-hijacked responses.
type HijackableResponseRecorder struct {
	wrapped *httptest.ResponseRecorder
	in      *bufio.Reader
	out     *bufio.Writer
	conn    *mockconn.Conn
}

func (h *HijackableResponseRecorder) Header() http.Header {
	return h.wrapped.Header()
}

func (h *HijackableResponseRecorder) WriteHeader(code int) {
	h.wrapped.WriteHeader(code)
}

func (h *HijackableResponseRecorder) Write(buf []byte) (int, error) {
	return h.wrapped.Write(buf)
}

func (h *HijackableResponseRecorder) WriteString(str string) (int, error) {
	return h.wrapped.WriteString(str)
}

func (h *HijackableResponseRecorder) Flush() {
	h.wrapped.Flush()
}

func (h *HijackableResponseRecorder) Result() *http.Response {
	return h.wrapped.Result()
}

func (h *HijackableResponseRecorder) Body() *bytes.Buffer {
	return h.wrapped.Body
}

func (h *HijackableResponseRecorder) Code() int {
	return h.wrapped.Code
}

func (h *HijackableResponseRecorder) Flushed() bool {
	return h.wrapped.Flushed
}

func (h *HijackableResponseRecorder) Closed() bool {
	return h.conn.Closed()
}

func (h *HijackableResponseRecorder) HeaderMap() http.Header {
	return h.wrapped.HeaderMap
}

func (h *HijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return h.conn, bufio.NewReadWriter(h.in, h.out), nil
}
