package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_copyHeader(t *testing.T) {
	destHeader := make(http.Header)
	srcHeader := make(http.Header)
	srcHeader.Add("X-Foo", "Foo")
	srcHeader.Add("X-Bar", "Bar")

	copyHeader(destHeader, srcHeader)
	assert.Equal(t, destHeader.Get("X-Foo"), srcHeader.Get("X-Foo"))
	assert.Equal(t, destHeader.Get("X-Bar"), srcHeader.Get("X-Bar"))
}

func Test_delHopHeaders(t *testing.T) {
	h := make(http.Header)
	h.Add("X-Foo", "Foo")
	h.Add("Upgrade", "websocket")

	delHopHeaders(h)
	assert.Empty(t, h.Get("Upgrade"))
}

func Test_appendHostToXForwardHeader(t *testing.T) {
	h := make(http.Header)
	h.Set("X-Forwarded-For", "prior.sn.natemurthy")
	appendHostToXForwardHeader(h, "api.sn.natemurthy")
	assert.Equal(t, "prior.sn.natemurthy, api.sn.natemurthy", h.Get("X-Forwarded-For"))
}

func Test_isPrivate(t *testing.T) {
	openEndpoints = []string{"/ping", "/metrics"}
	assert.True(t, isPrivate("/private"))
	assert.False(t, isPrivate("/ping"))
}

func Test_isAuthenticated(t *testing.T) {
	basicAuthAllowed = []string{"user:pass", "foo:bar"}

	r, _ := http.NewRequest("GET", "/protected/data", nil)

	r.SetBasicAuth("user", "pass")
	assert.True(t, isAuthenticated(r))

	r.SetBasicAuth("fud", "dud")
	assert.False(t, isAuthenticated(r))
}

type readerCloser struct {
	*bytes.Buffer
}

func (r *readerCloser) Close() error { return nil }

type mockHTTPClient struct {
	test          *testing.T
	sourceRequest *http.Request
}

func (m *mockHTTPClient) setRequest(r *http.Request) {
	m.sourceRequest = r
}

func (m *mockHTTPClient) Do(r *http.Request) (*http.Response, error) {

	// use this to test inboundReq and outboundReq headers are copied
	assert.Equal(
		m.test,
		m.sourceRequest.Header.Get("Copied-Header"),
		r.Header.Get("Copied-Header"))

	b := new(bytes.Buffer)

	if r.URL.Hostname() == "server.error" {
		return &http.Response{}, fmt.Errorf("Server Error")
	}
	if r.URL.Query().Get("start") == "now" {
		b.WriteString("now")
		respBody := &readerCloser{b}
		return &http.Response{StatusCode: http.StatusOK, Body: respBody}, nil
	}
	b.WriteString("ok")
	respBody := &readerCloser{b}
	return &http.Response{StatusCode: http.StatusOK, Body: respBody}, nil
}

func TestServeHTTP(t *testing.T) {

	basicAuthAllowed = []string{"user:pass", "foo:bar"}
	openEndpoints = []string{"/ping", "/public"}

	m := &mockHTTPClient{test: t}
	p := proxy{m}

	r1, _ := http.NewRequest("GET", "/public", nil)
	m.setRequest(r1)
	w1 := httptest.NewRecorder()
	p.ServeHTTP(w1, r1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, "ok", w1.Body.String())

	r2, _ := http.NewRequest("GET", "/private", nil)
	m.setRequest(r2)
	w2 := httptest.NewRecorder()
	p.ServeHTTP(w2, r2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	r3, _ := http.NewRequest("GET", "/private", nil)
	m.setRequest(r3)
	r3.SetBasicAuth("user", "pass")
	w3 := httptest.NewRecorder()
	p.ServeHTTP(w3, r3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, "ok", w3.Body.String())

	r4, _ := http.NewRequest("GET", "/private?start=now", nil)
	m.setRequest(r4)
	r4.SetBasicAuth("user", "pass")
	w4 := httptest.NewRecorder()
	p.ServeHTTP(w4, r4)
	assert.Equal(t, http.StatusOK, w4.Code)
	assert.Equal(t, "now", w4.Body.String())

	r5, _ := http.NewRequest("POST", "/public", bytes.NewReader([]byte("data")))
	r5.Header.Set("Copied-Header", "foo/bar")
	m.setRequest(r5)
	w5 := httptest.NewRecorder()
	p.ServeHTTP(w5, r5)

	// -- error cases below --

	*dest = "http://server.error"
	r6, _ := http.NewRequest("GET", "/public", nil)
	m.setRequest(r6)
	w6 := httptest.NewRecorder()
	p.ServeHTTP(w6, r6)
	assert.Equal(t, http.StatusInternalServerError, w6.Code)

	*dest = "wss://unsuported"
	r7, _ := http.NewRequest("GET", "/public", nil)
	m.setRequest(r7)
	w7 := httptest.NewRecorder()
	p.ServeHTTP(w7, r7)
	assert.Equal(t, http.StatusBadRequest, w7.Code)
}
