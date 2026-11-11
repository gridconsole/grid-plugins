package env

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/ports"
)

// Manager owns only local port records and dev processes. It never writes the
// Job lease or Feed. Dir is the checkout where the environment command runs.
// Grace applies separately after SIGINT and SIGTERM; zero uses two seconds.
type Manager struct {
	Ports   ports.Store
	TaskDir string
	Dir     string
	Grace   time.Duration
	// OwnerPID is the process the port claim names as its owner; zero is this process.
	OwnerPID int
}

func (m Manager) grace() time.Duration {
	if m.Grace > 0 {
		return m.Grace
	}
	return 2 * time.Second
}

// Up returns only after a 2xx health response and a fresh ownership check.
// Startup and pid handoff hold the allocator lock; health polling does not.
func (m Manager) Up(ctx context.Context, id string, c Config) (ports.Record, error) {
	if err := supported(); err != nil {
		return ports.Record{}, err
	}
	timeout, err := c.Validate()
	if err != nil {
		return ports.Record{}, err
	}
	if m.Dir == "" {
		return ports.Record{}, errors.New("a project checkout directory is required")
	}
	if err := ctx.Err(); err != nil {
		return ports.Record{}, err
	}
	r, done, err := m.start(id, c)
	if err != nil {
		return r, err
	}
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	healthErr := waitHealth(healthCtx, c.HealthURL, r.Start, done)
	// Down, gc or another up can replace the reservation while health is pending.
	// Never stop or remove that replacement on behalf of this invocation.
	l, err := m.Ports.Lock()
	if err != nil {
		return r, errors.Join(healthErr, err)
	}
	defer l.Close()
	current, found, err := l.Find(m.TaskDir, id)
	if err != nil {
		return r, errors.Join(healthErr, err)
	}
	if !found || current != r {
		return r, errors.Join(healthErr, errors.New("dev server reservation changed while waiting for health"))
	}
	matches, err := ports.ProcessMatches(r)
	if err != nil {
		return r, errors.Join(healthErr, err)
	}
	if !matches {
		return r, errors.Join(healthErr, errors.New("dev server exited or its process identity changed while waiting for health"), l.Remove(r))
	}
	if healthErr != nil {
		if err := ports.StopRecord(r, m.grace()); err != nil {
			return r, errors.Join(healthErr, err)
		}
		return r, errors.Join(healthErr, l.Remove(r))
	}
	return r, nil
}

// start serializes only the claim, process launch and pid handoff.
func (m Manager) start(id string, c Config) (ports.Record, <-chan error, error) {
	l, err := m.Ports.Lock()
	if err != nil {
		return ports.Record{}, nil, err
	}
	defer l.Close()
	old, found, err := l.Find(m.TaskDir, id)
	if err != nil {
		return ports.Record{}, nil, err
	}
	if found && old.Group {
		matches, err := ports.ProcessMatches(old)
		if err != nil {
			return old, nil, err
		}
		if matches {
			return old, nil, nil
		}
		if err := l.Remove(old); err != nil {
			return old, nil, err
		}
	}
	rnge, _ := c.Ports()
	owner := m.OwnerPID
	if owner == 0 {
		owner = os.Getpid()
	}
	r, err := l.Claim(m.TaskDir, id, owner, rnge)
	if err != nil {
		return r, nil, err
	}
	if !r.Range.Available() {
		return r, nil, errors.Join(errors.New("reserved dev ports already have a listener; stop it or choose another port range"), l.Remove(r))
	}
	port := strconv.Itoa(r.Start)
	cmd := exec.Command("/bin/sh", "-c", strings.ReplaceAll(c.Command, "{port}", port))
	cmd.Dir = m.Dir
	cmd.Env = devEnvironment(port, id)
	if err := configure(cmd); err != nil {
		return r, nil, errors.Join(err, l.Remove(r))
	}
	// A file avoids inherited CLI pipes and lets the detached server keep running.
	log, err := fslock.OpenNoFollow(filepath.Join(m.Ports.GridHome, "ports", "env-"+port+".log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return r, nil, errors.Join(err, l.Remove(r))
	}
	cmd.Stdout, cmd.Stderr = log, log
	err = cmd.Start()
	_ = log.Close()
	if err != nil {
		return r, nil, errors.Join(err, l.Remove(r))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	r, err = l.SetProcess(r, cmd.Process.Pid)
	if err != nil {
		if stopErr := stop(cmd.Process.Pid, m.grace()); stopErr != nil {
			return r, nil, errors.Join(err, stopErr)
		}
		return r, nil, errors.Join(err, l.Remove(r))
	}
	return r, done, nil
}

// devEnvironment starts from an explicit allowlist. Shell startup hooks, agent
// settings, API keys and arbitrary parent variables never enter the dev command.
func devEnvironment(port, job string) []string {
	env := []string{}
	for _, key := range []string{"PATH", "HOME", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "LC_CTYPE", "LC_COLLATE", "LC_MESSAGES", "LC_MONETARY", "LC_NUMERIC", "LC_TIME", "TERM", "TZ"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return append(env, "PORT="+port, "GRID_PORT="+port, "GRID_JOB="+job)
}

// Down is idempotent. It retains the reservation if process signalling fails.
func (m Manager) Down(id string) error {
	if err := supported(); err != nil {
		return err
	}
	l, err := m.Ports.Lock()
	if err != nil {
		return err
	}
	defer l.Close()
	r, found, err := l.Find(m.TaskDir, id)
	if err != nil || !found {
		return err
	}
	if r.Group {
		if err := ports.StopRecord(r, m.grace()); err != nil {
			return err
		}
	}
	return l.Remove(r)
}

func waitHealth(ctx context.Context, template string, port int, done <-chan error) error {
	url := strings.ReplaceAll(template, "{port}", strconv.Itoa(port))
	// Local health checks must not inherit proxy routing or follow a redirect to
	// an unrelated healthy page. Never retain idle sockets into shutdown.
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return fmt.Errorf("dev server exited before becoming healthy: %v", err)
		case <-ctx.Done():
			return fmt.Errorf("dev server health check: %w", ctx.Err())
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err == nil {
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				select {
				case err := <-done:
					return fmt.Errorf("dev server exited before becoming healthy: %v", err)
				default:
					return nil
				}
			}
		}
		select {
		case err := <-done:
			return fmt.Errorf("dev server exited before becoming healthy: %v", err)
		case <-ctx.Done():
			return fmt.Errorf("dev server health check: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
