package devproxy

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/umono-cms/cli/internal/devserver"
)

// sessionCookie is the cookie of Umono's session.
const sessionCookie = "session_id"

// loginClient logs in to .dev/server. It does not follow the redirect of
// the login.
var loginClient = &http.Client{
	Timeout: 10 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// errLogin is returned when .dev/server gives no session to the account of
// .dev/server.
var errLogin = errors.New("logging in to .dev/server failed")

// session is the proxy's own login to .dev/server with the account of
// .dev/server. The browser never gets its cookie: the proxy adds it to the
// /admin requests that have no session of their own.
type session struct {
	mu     sync.Mutex
	cookie *http.Cookie
}

// get returns the session's cookie, logging in to target first when there
// is none.
func (s *session) get(ctx context.Context, target *url.URL) (*http.Cookie, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cookie != nil {
		return s.cookie, nil
	}
	form := url.Values{"username": {devserver.Username}, "password": {devserver.Password}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target.JoinPath("/login").String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := loginClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			s.cookie = &http.Cookie{Name: c.Name, Value: c.Value}
			return s.cookie, nil
		}
	}
	return nil, errLogin
}

// forget drops the session when its cookie still has value, so the next
// /admin request logs in again.
func (s *session) forget(value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cookie != nil && s.cookie.Value == value {
		s.cookie = nil
	}
}

// loggedOut reports whether Umono did not take the proxy's session, as
// after a restart of .dev/server or a logout: it sends the request to the
// login page, or shows the login page.
func loggedOut(resp *http.Response) bool {
	if resp.Request.URL.Path == "/admin/login" && resp.StatusCode == http.StatusOK {
		return true
	}
	return isLoginURL(resp.Header.Get("Location")) || isLoginURL(resp.Header.Get("HX-Redirect"))
}

// isLoginURL reports whether s is the URL of the login page.
func isLoginURL(s string) bool {
	u, err := url.Parse(s)
	return s != "" && err == nil && u.Path == "/admin/login"
}

// toAdmin turns resp into a redirect to /admin, where the proxy logs in
// again: an HX-Redirect for htmx, a 303 otherwise.
func toAdmin(resp *http.Response) {
	resp.Body.Close()
	h := http.Header{}
	if resp.Request.Header.Get("HX-Request") == "true" {
		resp.StatusCode = http.StatusOK
		h.Set("HX-Redirect", "/admin")
	} else {
		resp.StatusCode = http.StatusSeeOther
		h.Set("Location", "/admin")
	}
	resp.Status = http.StatusText(resp.StatusCode)
	h.Set("Content-Length", "0")
	resp.Header = h
	resp.Body = http.NoBody
	resp.ContentLength = 0
}
