// Package themedev runs umono theme dev: .dev/server with the theme in
// development, behind the dev proxy, following the changes of the theme
// project.
package themedev

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/umono-cms/cli/internal/compatibility"
	"github.com/umono-cms/cli/internal/devproxy"
	"github.com/umono-cms/cli/internal/devserver"
	"github.com/umono-cms/cli/internal/devtheme"
	"github.com/umono-cms/cli/internal/devwatch"
	"github.com/umono-cms/cli/internal/download"
	"github.com/umono-cms/cli/internal/themeproject"
	"github.com/umono-cms/cli/internal/umonoproject"
	"github.com/umono-cms/themecore"
)

// DefaultPort is the port of the dev proxy without --port.
const DefaultPort = 8990

// Options of Run.
type Options struct {
	// ProjectDir is the theme project.
	ProjectDir string
	// Port is the dev proxy's.
	Port int
	// UpdateServer runs the upgrade flow on .dev/server first.
	UpdateServer bool
	// Client gets the releases of Umono.
	Client *download.Client
	// Registry is where the contracts are fetched from.
	Registry themeproject.Registry
	// Out gets the messages of dev.
	Out io.Writer
	// Interval is how often the theme project is scanned for changes.
	Interval time.Duration
}

// Run runs umono theme dev in o.ProjectDir until ctx is done. First it gets
// the contract of theme.toml, and stops without it. Then .dev/server is set
// up with the latest release when it is missing, or updated with
// UpdateServer, its Theme API range is compared with the CLI's, the dev
// directory is brought up to date and the server starts behind the dev
// proxy. Every change of the project brings the dev directory up to date
// again and tells the pages to reload, to swap the stylesheet or to show the
// errors. A new contract restarts .dev/server, which reads it only at
// startup.
func Run(ctx context.Context, o Options) error {
	dir := o.ProjectDir
	b, err := os.ReadFile(filepath.Join(dir, "theme.toml"))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s: %w", dir, themeproject.ErrNotThemeProject)
	}
	if err != nil {
		return err
	}
	m, err := themecore.ParseManifest(b)
	if err != nil {
		return err
	}
	if _, err := themeproject.EnsureContract(ctx, dir, m.Contract, o.Registry); err != nil {
		return err
	}

	created, err := devserver.Ensure(dir, o.Client)
	if err != nil {
		return err
	}
	if o.UpdateServer && !created {
		if err := devserver.Update(dir, o.Client); err != nil {
			return err
		}
	}
	warning, err := devserver.CheckThemeAPI(dir)
	switch {
	case errors.Is(err, umonoproject.ErrNoThemeSupport):
		return fmt.Errorf("%w; update .dev/server with: umono theme dev --update-server", err)
	case errors.Is(err, devserver.ErrCLIOutdated):
		return fmt.Errorf("%w; upgrade the CLI: %s", err, compatibility.CLIUpgradeURL)
	case err != nil:
		return err
	}
	if warning != "" {
		fmt.Fprintf(o.Out, "warning: %s\n", warning)
	}

	serverDir := devserver.Dir(dir)
	state, err := devtheme.Sync(ctx, dir, serverDir, o.Registry, time.Now())
	if err != nil {
		return err
	}
	report(o.Out, state)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", o.Port))
	if err != nil {
		return fmt.Errorf("the port %d of the dev proxy is in use, choose another with --port: %w", o.Port, err)
	}
	srv, err := devserver.Start(dir)
	if err != nil {
		ln.Close()
		return err
	}
	defer func() { srv.Stop() }()
	proxy, err := devproxy.New(srv.URL)
	if err != nil {
		ln.Close()
		return err
	}
	proxy.Publish(event(state, "reload"))
	web := &http.Server{Handler: proxy, ReadHeaderTimeout: 10 * time.Second}
	go web.Serve(ln)
	// Close, not Shutdown: the event streams never become idle.
	defer web.Close()
	fmt.Fprintf(o.Out, "Developing %s at http://localhost:%d, admin at http://localhost:%d/admin\n", m.Name, o.Port, o.Port)

	contract := state.Contract
	devwatch.Watch(ctx, dir, o.Interval, func(changed []string) {
		state, err := devtheme.Sync(ctx, dir, serverDir, o.Registry, time.Now())
		if err != nil {
			fmt.Fprintf(o.Out, "error: %v\n", err)
			proxy.Publish(devproxy.Event{Name: "error", Errors: []string{err.Error()}})
			return
		}
		report(o.Out, state)
		if state.Contract != contract {
			fmt.Fprintf(o.Out, "The contract is %s now, restarting .dev/server\n", state.Contract)
			srv.Stop()
			next, err := devserver.Start(dir)
			if err != nil {
				fmt.Fprintf(o.Out, "error: %v\n", err)
				proxy.Publish(devproxy.Event{Name: "error", Errors: []string{err.Error()}})
				return
			}
			srv, contract = next, state.Contract
			proxy.SetTarget(srv.URL)
			proxy.Publish(event(state, "reload"))
			return
		}
		name := "reload"
		if devwatch.OnlyCSS(changed) {
			name = "css"
		}
		proxy.Publish(event(state, name))
	})
	return nil
}

// event is the event of state: name when the checks pass, error otherwise.
func event(state devtheme.State, name string) devproxy.Event {
	if state.Err != nil {
		return devproxy.Event{Name: "error", Errors: []string{state.Err.Error()}}
	}
	e := devproxy.Event{Name: name}
	for _, w := range state.Warnings {
		e.Warnings = append(e.Warnings, w.Where+": "+w.Message)
	}
	return e
}

// report prints the outcome of the checks, as build does.
func report(out io.Writer, state devtheme.State) {
	if state.Err != nil {
		fmt.Fprintf(out, "error: %v\n", state.Err)
		return
	}
	for _, w := range state.Warnings {
		fmt.Fprintf(out, "warning: %s: %s\n", w.Where, w.Message)
	}
}
