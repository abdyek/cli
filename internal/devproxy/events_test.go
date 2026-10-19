package devproxy

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func stream(t *testing.T, p *Proxy) func() (name, data string) {
	t.Helper()
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/_umono-dev/events", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want prefix %q", ct, "text/event-stream")
	}
	r := bufio.NewReader(resp.Body)
	return func() (name, data string) {
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			line = strings.TrimRight(line, "\r\n")
			if line == "" {
				return name, data
			}
			if strings.HasPrefix(line, ":") {
				continue
			}
			switch {
			case strings.HasPrefix(line, "event: "):
				name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data = strings.TrimPrefix(line, "data: ")
			}
		}
	}
}

func TestEventsSendTheStateFirst(t *testing.T) {
	p := newProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("forwarded") })
	p.Publish(Event{Name: "error", Errors: []string{"theme.css: bad"}})
	next := stream(t, p)
	name, data := next()
	if name != "state" {
		t.Fatalf("name = %q, want %q", name, "state")
	}
	if data != `{"errors":["theme.css: bad"],"warnings":[]}` {
		t.Fatalf("data = %q, want %q", data, `{"errors":["theme.css: bad"],"warnings":[]}`)
	}
}

func TestEventsStreamWhatIsPublished(t *testing.T) {
	p := newProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("forwarded") })
	next := stream(t, p)
	name, data := next()
	if name != "state" {
		t.Fatalf("name = %q, want %q", name, "state")
	}
	if data != `{"errors":[],"warnings":[]}` {
		t.Fatalf("data = %q, want %q", data, `{"errors":[],"warnings":[]}`)
	}
	p.Publish(Event{Name: "css", Warnings: []string{"dead address"}})
	name, data = next()
	if name != "css" {
		t.Fatalf("name = %q, want %q", name, "css")
	}
	if data != `{"errors":[],"warnings":["dead address"]}` {
		t.Fatalf("data = %q, want %q", data, `{"errors":[],"warnings":["dead address"]}`)
	}
	p.Publish(Event{Name: "reload"})
	name, _ = next()
	if name != "reload" {
		t.Fatalf("name = %q, want %q", name, "reload")
	}
}

func TestClientFilesAreServed(t *testing.T) {
	cases := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/_umono-dev/client.js", "text/javascript; charset=utf-8", "EventSource"},
		{"/_umono-dev/overlay.css", "text/css; charset=utf-8", "umono"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.path, func(t *testing.T) {
			p := newProxy(t, func(w http.ResponseWriter, r *http.Request) { t.Error("forwarded") })
			resp, body := get(t, p, c.path, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if got := resp.Header.Get("Content-Type"); got != c.contentType {
				t.Fatalf("Content-Type = %q, want %q", got, c.contentType)
			}
			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
			}
			if !strings.Contains(body, c.contains) {
				t.Fatalf("body does not contain %q", c.contains)
			}
		})
	}
}
