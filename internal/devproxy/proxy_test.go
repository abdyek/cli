package devproxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const page = "<html><head><title>x</title></head><body>hi</body></html>"

const csp = "default-src 'none'; script-src 'nonce-abc'; style-src 'self'"

func get(t *testing.T, p http.Handler, path string, h http.Header) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	res := rec.Result()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	res.Body.Close()
	return res, string(body)
}

func newProxy(t *testing.T, upstream http.HandlerFunc) *Proxy {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	p, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func TestProxyForwardsWithTheHostAndWithoutCompression(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Host + "|" + r.Header.Get("Accept-Encoding")))
	}
	p := newProxy(t, upstream)
	_, body := get(t, p, "http://localhost:8990/about", http.Header{"Accept-Encoding": {"gzip"}})
	if body != "localhost:8990|identity" {
		t.Fatalf("body = %q, want %q", body, "localhost:8990|identity")
	}
}

func TestProxyInjectsTheClientScript(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("ETag", `W/"x"`)
		_, _ = w.Write([]byte(page))
	}
	p := newProxy(t, upstream)
	want := `<html><head><title>x</title><script nonce="abc" src="/_umono-dev/client.js" defer></script></head><body>hi</body></html>`
	resp, body := get(t, p, "/", nil)
	if body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
	if got := resp.Header.Get("Content-Length"); got != strconv.Itoa(len(want)) {
		t.Fatalf("Content-Length = %q, want %q", got, strconv.Itoa(len(want)))
	}
	if got := resp.Header.Get("ETag"); got != "" {
		t.Fatalf("ETag = %q, want %q", got, "")
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != csp+"; connect-src 'self'" {
		t.Fatalf("CSP = %q, want %q", got, csp+"; connect-src 'self'")
	}
}

func TestProxyAddsSelfToConnectSrc(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"default-src 'none'; connect-src https://a.example", "default-src 'none'; connect-src https://a.example 'self'"},
		{"default-src 'none'; connect-src 'self'", "default-src 'none'; connect-src 'self'"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			upstream := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.Header().Set("Content-Security-Policy", c.in)
				_, _ = w.Write([]byte(page))
			}
			p := newProxy(t, upstream)
			resp, _ := get(t, p, "/", nil)
			if got := resp.Header.Get("Content-Security-Policy"); got != c.want {
				t.Fatalf("CSP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProxyWithoutANonceAddsNoScript(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Security-Policy", "default-src 'none'")
		w.Header().Set("ETag", `W/"x"`)
		_, _ = w.Write([]byte(page))
	}
	p := newProxy(t, upstream)
	resp, body := get(t, p, "/", nil)
	if body != page {
		t.Fatalf("body = %q, want %q", body, page)
	}
	if got := resp.Header.Get("ETag"); got != `W/"x"` {
		t.Fatalf("ETag = %q, want %q", got, `W/"x"`)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != "default-src 'none'; connect-src 'self'" {
		t.Fatalf("CSP = %q, want %q", got, "default-src 'none'; connect-src 'self'")
	}
}

func TestProxyLeavesOtherResponses(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("ETag", `"y"`)
		_, _ = w.Write([]byte("a{}"))
	}
	p := newProxy(t, upstream)
	resp, body := get(t, p, "/", nil)
	if body != "a{}" {
		t.Fatalf("body = %q, want %q", body, "a{}")
	}
	if got := resp.Header.Get("ETag"); got != `"y"` {
		t.Fatalf("ETag = %q, want %q", got, `"y"`)
	}
}

func TestProxyKeepsItsOwnPaths(t *testing.T) {
	count := 0
	upstream := func(w http.ResponseWriter, r *http.Request) {
		count++
	}
	p := newProxy(t, upstream)
	resp, _ := get(t, p, "/_umono-dev/unknown", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	if count != 0 {
		t.Fatalf("upstream count = %d, want 0", count)
	}
}
