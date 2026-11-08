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
}

func (m Manager) grace() time.Duration {
	if m.Grace > 0 {
		return m.Grace
	}
	return 2 * time.Second
}

// Up returns only after a 2xx health response. The allocator transaction spans
// startup so another up/down/gc cannot race the pid handoff. Long starts thus
// serialize local allocator operations, bounded by the configured timeout.
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
	l, err := m.Ports.Lock()
	if err != nil {
		return ports.Record{}, err
	}
	defer l.Close()
	old, found, err := l.Find(m.TaskDir, id)
	if err != nil {
		return ports.Record{}, err
	}
	if found && old.Group {
		matches, err := ports.ProcessMatches(old)
		if err != nil {
			return old, err
		}
		if matches {
			healthCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			err := waitHealth(healthCtx, c.HealthURL, old.Start, nil)
			if err != nil {
				if stopErr := ports.StopRecord(old, m.grace()); stopErr != nil {
					return old, errors.Join(err, stopErr)
				}
				return old, errors.Join(err, l.Remove(old))
			}
			return old, nil
		}
		if err := l.Remove(old); err != nil {
			return old, err
		}
	}
	rnge, _ := c.Ports()
	r, err := l.Claim(m.TaskDir, id, os.Getpid(), rnge)
	if err != nil {
		return r, err
	}
	if !r.Range.Available() {
		return r, errors.Join(errors.New("reserved dev ports already have a listener; stop it or choose another port range"), l.Remove(r))
	}
	port := strconv.Itoa(r.Start)
	cmd := exec.Command("/bin/sh", "-c", strings.ReplaceAll(c.Command, "{port}", port))
	cmd.Dir = m.Dir
	cmd.Env = append(os.Environ(), "PORT="+port, "GRID_PORT="+port, "GRID_JOB="+id)
	if err := configure(cmd); err != nil {
		return r, errors.Join(err, l.Remove(r))
	}
	// A file avoids inherited CLI pipes and lets the detached server keep running.
	log, err := fslock.OpenNoFollow(filepath.Join(m.Ports.GridHome, "ports", "env-"+port+".log"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return r, errors.Join(err, l.Remove(r))
	}
	cmd.Stdout, cmd.Stderr = log, log
	err = cmd.Start()
	_ = log.Close()
	if err != nil {
		return r, errors.Join(err, l.Remove(r))
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	cleanup := func(cause error) (ports.Record, error) {
		stopErr := stop(cmd.Process.Pid, m.grace())
		if stopErr != nil {
			return r, errors.Join(cause, stopErr)
		}
		return r, errors.Join(cause, l.Remove(r))
	}
	r, err = l.SetProcess(r, cmd.Process.Pid)
	if err != nil {
		return cleanup(err)
	}
	healthCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := waitHealth(healthCtx, c.HealthURL, r.Start, done); err != nil {
		return cleanup(err)
	}
	return r, nil
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
