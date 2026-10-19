// Package devproxy is the dev proxy of umono theme dev. It forwards every
// request to .dev/server and changes the responses only as live reload and
// the error overlay need.
package devproxy

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
)

// prefix starts the proxy's own paths. Umono never uses it, so the proxy
// does not forward these requests.
const prefix = "/_umono-dev/"

// clientScript is the only tag the proxy adds to a page.
const clientScript = prefix + "client.js"

// Proxy forwards requests to .dev/server. It is an http.Handler.
type Proxy struct {
	target  atomic.Pointer[url.URL]
	rp      *httputil.ReverseProxy
	session session
}

// sessionKey marks an outgoing request that carries the proxy's session,
// with its value.
type sessionKey struct{}

// New returns a proxy to target, the URL of .dev/server.
func New(target string) (*Proxy, error) {
	p := &Proxy{}
	if err := p.SetTarget(target); err != nil {
		return nil, err
	}
	p.rp = &httputil.ReverseProxy{Rewrite: p.rewrite, ModifyResponse: p.modifyResponse}
	return p, nil
}

// SetTarget points the proxy at target, the URL of .dev/server, which
// changes when .dev/server restarts on another port.
func (p *Proxy) SetTarget(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL of .dev/server: %q", target)
	}
	p.target.Store(u)
	return nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, prefix) {
		http.NotFound(w, r)
		return
	}
	p.rp.ServeHTTP(w, r)
}

// rewrite sends the request to .dev/server with the browser's Host, so
// Umono sees the page's own origin, and asks for an uncompressed body,
// which the proxy can change. An /admin request without a session of the
// browser gets the proxy's own.
func (p *Proxy) rewrite(pr *httputil.ProxyRequest) {
	target := p.target.Load()
	pr.SetURL(target)
	pr.Out.Host = pr.In.Host
	pr.Out.Header.Set("Accept-Encoding", "identity")
	if !isAdmin(pr.In.URL.Path) {
		return
	}
	if _, err := pr.In.Cookie(sessionCookie); err == nil {
		return
	}
	c, err := p.session.get(pr.In.Context(), target)
	if err != nil {
		return
	}
	pr.Out.AddCookie(c)
	pr.Out = pr.Out.WithContext(context.WithValue(pr.Out.Context(), sessionKey{}, c.Value))
}

// modifyResponse adds connect-src 'self' to the CSP, which the SSE of live
// reload needs, and the client script to an HTML page with a script-src
// nonce. A changed body gets its Content-Length and loses its ETag. A
// response that did not take the proxy's session logs in again.
func (p *Proxy) modifyResponse(resp *http.Response) error {
	if value, ok := resp.Request.Context().Value(sessionKey{}).(string); ok && loggedOut(resp) {
		p.session.forget(value)
		toAdmin(resp)
		return nil
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		return nil
	}
	resp.Header.Set("Content-Security-Policy", withConnectSelf(csp))
	nonce := scriptNonce(csp)
	if nonce == "" || !isHTML(resp) {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return err
	}
	if i := bytes.Index(body, []byte("</head>")); i >= 0 {
		tag := `<script nonce="` + html.EscapeString(nonce) + `" src="` + clientScript + `" defer></script>`
		body = slices.Concat(body[:i], []byte(tag), body[i:])
		resp.Header.Del("ETag")
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

// isAdmin reports whether path is under /admin.
func isAdmin(path string) bool {
	return path == "/admin" || strings.HasPrefix(path, "/admin/")
}

// isHTML reports whether resp is an HTML page.
func isHTML(resp *http.Response) bool {
	t, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	return err == nil && t == "text/html"
}

// withConnectSelf adds 'self' to connect-src of csp, or connect-src 'self'
// when it has none. Nothing else changes.
func withConnectSelf(csp string) string {
	directives := strings.Split(csp, ";")
	for i, d := range directives {
		fields := strings.Fields(d)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "connect-src") {
			continue
		}
		sources := fields[1:]
		if slices.Contains(sources, "'self'") {
			return csp
		}
		if len(sources) == 1 && sources[0] == "'none'" {
			directives[i] = " connect-src 'self'"
		} else {
			directives[i] = strings.TrimRight(d, " ") + " 'self'"
		}
		return strings.Join(directives, ";")
	}
	return strings.TrimSuffix(strings.TrimSpace(csp), ";") + "; connect-src 'self'"
}

// scriptNonce returns the nonce of script-src in csp, "" without one.
func scriptNonce(csp string) string {
	for _, d := range strings.Split(csp, ";") {
		fields := strings.Fields(d)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "script-src") {
			continue
		}
		for _, f := range fields[1:] {
			if v, ok := strings.CutPrefix(f, "'nonce-"); ok {
				return strings.TrimSuffix(v, "'")
			}
		}
	}
	return ""
}
