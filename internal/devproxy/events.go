package devproxy

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// client holds the files of /_umono-dev/: the live reload and overlay
// client and the overlay's stylesheet.
//
//go:embed client/client.js client/overlay.css
var client embed.FS

// keepAlive is how often an idle event stream gets a comment, so proxies
// and browsers keep it open.
const keepAlive = 30 * time.Second

// Event is a message of /_umono-dev/events. Name is "reload" (reload the
// page), "css" (only assets/theme.css changed, swap its stylesheet) or
// "error" (show the errors in the overlay).
type Event struct {
	Name     string
	Errors   []string
	Warnings []string
}

// state is the data of an event: what the overlay shows.
type state struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// broker sends the published events to the open streams.
type broker struct {
	mu      sync.Mutex
	last    state
	streams map[chan Event]struct{}
}

// Publish sends e to every open /_umono-dev/events. Its errors and warnings
// are the state that a new stream gets first, as a "state" event.
func (p *Proxy) Publish(e Event) {
	p.events.mu.Lock()
	defer p.events.mu.Unlock()
	p.events.last = state{Errors: e.Errors, Warnings: e.Warnings}
	for ch := range p.events.streams {
		select {
		case ch <- e:
		default:
		}
	}
}

// serveEvents streams the events as Server-Sent Events, the state first.
func (p *Proxy) serveEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	ch := make(chan Event, 16)
	p.events.mu.Lock()
	if p.events.streams == nil {
		p.events.streams = map[chan Event]struct{}{}
	}
	p.events.streams[ch] = struct{}{}
	first := p.events.last
	p.events.mu.Unlock()
	defer func() {
		p.events.mu.Lock()
		delete(p.events.streams, ch)
		p.events.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	writeEvent(w, "state", first)
	flusher.Flush()
	ticker := time.NewTicker(keepAlive)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			writeEvent(w, e.Name, state{Errors: e.Errors, Warnings: e.Warnings})
		case <-ticker.C:
			fmt.Fprint(w, ": keep-alive\n\n")
		}
		flusher.Flush()
	}
}

// writeEvent writes one event. Empty lists are written as [].
func writeEvent(w http.ResponseWriter, name string, s state) {
	if s.Errors == nil {
		s.Errors = []string{}
	}
	if s.Warnings == nil {
		s.Warnings = []string{}
	}
	data, _ := json.Marshal(s)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
}

// serveOwn serves the proxy's own paths: the event stream and the client
// files, all no-store. Every other path is 404.
func (p *Proxy) serveOwn(w http.ResponseWriter, r *http.Request) {
	var name, contentType string
	switch r.URL.Path {
	case prefix + "events":
		p.serveEvents(w, r)
		return
	case clientScript:
		name, contentType = "client/client.js", "text/javascript; charset=utf-8"
	case prefix + "overlay.css":
		name, contentType = "client/overlay.css", "text/css; charset=utf-8"
	default:
		http.NotFound(w, r)
		return
	}
	b, err := client.ReadFile(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}
