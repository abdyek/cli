package devserver

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// startTimeout bounds the wait for .dev/server to listen.
const startTimeout = 30 * time.Second

// stopTimeout is how long Stop waits after the interrupt before it kills the
// server.
const stopTimeout = 5 * time.Second

// Server is a running .dev/server.
type Server struct {
	// URL is where it listens, http://127.0.0.1:{port}.
	URL string

	cmd  *exec.Cmd
	done chan struct{} // closed when the process has exited
	err  error         // the result of Wait, set before done is closed
	stop sync.Once
}

// Start starts the binary of .dev/server in its directory on a free port,
// which the CLI picks on every start, with UMONO_THEME_DEV set to the
// absolute path of projectDir: Umono opens its dev mode only with it. The
// server's output goes to .dev/server/umono.log. Start returns when the
// server listens. A server that exits before shows the end of its log.
func Start(projectDir string) (*Server, error) {
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, err
	}
	dir := Dir(root)
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	logPath := filepath.Join(dir, "umono.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	// The process has its own descriptor of the log.
	defer logFile.Close()

	cmd := exec.Command(filepath.Join(dir, "umono"))
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PORT="+strconv.Itoa(port), "UMONO_THEME_DEV="+root)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Server{URL: fmt.Sprintf("http://127.0.0.1:%d", port), cmd: cmd, done: make(chan struct{})}
	go func() {
		s.err = cmd.Wait()
		close(s.done)
	}()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(startTimeout)
	for {
		if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			conn.Close()
			return s, nil
		}
		select {
		case <-s.done:
			return nil, fmt.Errorf(".dev/server exited: %v\n%s", s.err, logTail(logPath))
		default:
		}
		if time.Now().After(deadline) {
			s.Stop()
			return nil, fmt.Errorf(".dev/server did not listen within %s\n%s", startTimeout, logTail(logPath))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Stop interrupts the server and kills it after stopTimeout. It can be
// called more than once.
func (s *Server) Stop() error {
	s.stop.Do(func() {
		select {
		case <-s.done:
			return
		default:
		}
		_ = s.cmd.Process.Signal(os.Interrupt)
		select {
		case <-s.done:
		case <-time.After(stopTimeout):
			_ = s.cmd.Process.Kill()
			<-s.done
		}
	})
	return nil
}

// freePort returns a port that is free on 127.0.0.1.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// logTail returns the last 20 lines of the log at path.
func logTail(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	return strings.Join(lines, "\n")
}
