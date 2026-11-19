package devproxy

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type fakeUmono struct {
	mu      sync.Mutex
	logins  int
	current string
}

func (f *fakeUmono) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/login":
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if r.PostForm.Get("username") == "admin" && r.PostForm.Get("password") == "admin" {
			f.mu.Lock()
			f.logins++
			f.current = "session" + strconv.Itoa(f.logins)
			current := f.current
			f.mu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: "session_id", Value: current, Path: "/admin"})
			w.Header().Set("HX-Redirect", "/admin")
			return
		}
		_, _ = w.Write([]byte("invalid"))
	case r.Method == http.MethodGet && r.URL.Path == "/admin/login":
		_, _ = w.Write([]byte("login page"))
	case strings.HasPrefix(r.URL.Path, "/admin"):
		f.mu.Lock()
		current := f.current
		f.mu.Unlock()
		c, err := r.Cookie("session_id")
		if err == nil && current != "" && c.Value == current {
			_, _ = w.Write([]byte("ok " + c.Value))
			return
		}
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	default:
		_, _ = w.Write([]byte("public"))
	}
}

func (f *fakeUmono) forget() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current = ""
}

func TestProxyLogsInToAdmin(t *testing.T) {
	f := &fakeUmono{}
	p := newProxy(t, f.ServeHTTP)
	resp, body := get(t, p, "/admin", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body != "ok session1" {
		t.Fatalf("body = %q, want %q", body, "ok session1")
	}
	_, body = get(t, p, "/admin/site-pages", nil)
	if body != "ok session1" {
		t.Fatalf("body = %q, want %q", body, "ok session1")
	}
	if f.logins != 1 {
		t.Fatalf("logins = %d, want 1", f.logins)
	}
	if got := resp.Header.Values("Set-Cookie"); len(got) != 0 {
		t.Fatalf("Set-Cookie = %v, want empty", got)
	}
}

func TestProxyUsesItsOwnSession(t *testing.T) {
	f := &fakeUmono{}
	var mu sync.Mutex
	var got []*http.Cookie
	upstream := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			mu.Lock()
			got = r.Cookies()
			mu.Unlock()
		}
		f.ServeHTTP(w, r)
	}
	p := newProxy(t, upstream)
	resp, body := get(t, p, "/admin", http.Header{"Cookie": {"theme=dark; session_id=mine"}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if body != "ok session1" {
		t.Fatalf("body = %q, want %q", body, "ok session1")
	}
	if f.logins != 1 {
		t.Fatalf("logins = %d, want 1", f.logins)
	}
	mu.Lock()
	cookies := got
	mu.Unlock()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %v, want 2", cookies)
	}
	values := map[string]string{}
	for _, c := range cookies {
		values[c.Name] = c.Value
		if c.Value == "mine" {
			t.Fatalf("cookie %q = %q, want no cookie of the browser", c.Name, c.Value)
		}
	}
	if values["theme"] != "dark" {
		t.Fatalf("theme cookie = %q, want %q", values["theme"], "dark")
	}
	if values["session_id"] != "session1" {
		t.Fatalf("session_id cookie = %q, want %q", values["session_id"], "session1")
	}
}

func TestProxyKeepsTheSessionCookieFromTheBrowser(t *testing.T) {
	upstream := func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "", Path: "/admin", MaxAge: -1})
		http.SetCookie(w, &http.Cookie{Name: "theme", Value: "dark", Path: "/"})
		_, _ = w.Write([]byte("ok"))
	}
	p := newProxy(t, upstream)
	for _, path := range []string{"/about", "/admin/logout"} {
		resp, _ := get(t, p, path, http.Header{"Cookie": {"session_id=mine"}})
		cookies := resp.Cookies()
		if len(cookies) != 1 {
			t.Fatalf("%s: cookies = %v, want 1, Set-Cookie = %v", path, cookies, resp.Header.Values("Set-Cookie"))
		}
		if cookies[0].Name != "theme" {
			t.Fatalf("%s: cookie name = %q, want %q, Set-Cookie = %v", path, cookies[0].Name, "theme", resp.Header.Values("Set-Cookie"))
		}
		if cookies[0].Value != "dark" {
			t.Fatalf("%s: theme cookie = %q, want %q, Set-Cookie = %v", path, cookies[0].Value, "dark", resp.Header.Values("Set-Cookie"))
		}
		for _, c := range cookies {
			if c.Name == "session_id" {
				t.Fatalf("%s: cookie %q = %q, want no cookie of the browser, Set-Cookie = %v", path, c.Name, c.Value, resp.Header.Values("Set-Cookie"))
			}
		}
	}
}

func TestProxyDoesNotLogInForPublicPages(t *testing.T) {
	f := &fakeUmono{}
	p := newProxy(t, f.ServeHTTP)
	_, body := get(t, p, "/about", nil)
	if body != "public" {
		t.Fatalf("body = %q, want %q", body, "public")
	}
	if f.logins != 0 {
		t.Fatalf("logins = %d, want 0", f.logins)
	}
}

func TestProxyLogsInAgainAfterARestart(t *testing.T) {
	f := &fakeUmono{}
	p := newProxy(t, f.ServeHTTP)
	_, body := get(t, p, "/admin", nil)
	if body != "ok session1" {
		t.Fatalf("body = %q, want %q", body, "ok session1")
	}
	f.forget()
	resp, _ := get(t, p, "/admin/site-pages", nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin" {
		t.Fatalf("Location = %q, want %q", loc, "/admin")
	}
	_, body = get(t, p, "/admin", nil)
	if body != "ok session2" {
		t.Fatalf("body = %q, want %q", body, "ok session2")
	}
	if f.logins != 2 {
		t.Fatalf("logins = %d, want 2", f.logins)
	}
}

func TestProxyLogsInAgainOnTheLoginPage(t *testing.T) {
	f := &fakeUmono{}
	p := newProxy(t, f.ServeHTTP)
	_, body := get(t, p, "/admin", nil)
	if body != "ok session1" {
		t.Fatalf("body = %q, want %q", body, "ok session1")
	}
	f.forget()
	resp, _ := get(t, p, "/admin/login", nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin" {
		t.Fatalf("Location = %q, want %q", loc, "/admin")
	}
	_, body = get(t, p, "/admin", nil)
	if body != "ok session2" {
		t.Fatalf("body = %q, want %q", body, "ok session2")
	}
}
