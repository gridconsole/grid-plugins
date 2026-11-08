package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	devenv "github.com/gridconsole/grid-plugins/internal/env"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/ports"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// environmentTask prefers the current task. From outside a task, a saved Job
// must identify exactly one task in the home, since J ids are task-scoped.
func environmentTask(e Env, id string) (task.Task, error) {
	if !job.ValidID(id) {
		return task.Task{}, errors.New("invalid Job id")
	}
	if e.GridHome == "" {
		return task.Task{}, errors.New("Grid home is required")
	}
	res, err := resolver(e).Resolve(where.Query{Cwd: e.Cwd})
	if err != nil {
		return task.Task{}, err
	}
	if res.TaskPath != "" {
		return task.Read(res.TaskPath)
	}
	dirs, err := gridhome.TasksDirs(e.GridHome)
	if err != nil {
		return task.Task{}, err
	}
	var found task.Task
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return found, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := os.Stat(filepath.Join(path, "jobs", id))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return found, err
			}
			if !info.IsDir() {
				continue
			}
			candidate, err := task.Read(path)
			if err != nil {
				return found, err
			}
			if found.Path != "" {
				return task.Task{}, fmt.Errorf("Job %s occurs in multiple tasks; run inside its task folder", id)
			}
			found = candidate
		}
	}
	if found.Path == "" {
		return found, fmt.Errorf("Job %s not found; run inside its task folder", id)
	}
	return found, nil
}

func runPorts(args []string, e Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(e.Stdout, "Usage: grid ports claim <J> [--json] | release <J> | list [--json] | gc")
		return 0
	}
	op := args[0]
	if op != "claim" && op != "release" && op != "list" && op != "gc" {
		fmt.Fprintf(e.Stderr, "ports: unknown command %q\n", op)
		return 2
	}
	fs := newFlags("ports "+op, e)
	asJSON := new(bool)
	if op == "claim" || op == "list" {
		asJSON = fs.Bool("json", false, "print JSON")
	}
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	count := 0
	if op == "claim" || op == "release" {
		count = 1
	}
	if len(operands) != count {
		fmt.Fprintf(e.Stderr, "ports %s: expected %d operands\n", op, count)
		return 2
	}
	s := ports.Store{GridHome: e.GridHome}
	if op == "list" || op == "gc" {
		var records []ports.Record
		var err error
		if op == "list" {
			records, err = s.List()
		} else {
			records, err = s.GC()
		}
		if err != nil {
			return commandError(e, "ports", err)
		}
		if *asJSON {
			return writeJSON(e.Stdout, records)
		}
		for _, r := range records {
			printPorts(e, r)
		}
		return 0
	}
	t, err := environmentTask(e, operands[0])
	if err != nil {
		return commandError(e, "ports", err)
	}
	if op == "release" {
		if err := s.Release(t.Path, operands[0]); err != nil {
			return commandError(e, "ports", err)
		}
		return 0
	}
	cfg := devenv.Config{}
	if t.Project != gridhome.Personal {
		cfg, _, err = devenv.Load(e.GridHome, t.Project)
		if err != nil {
			return commandError(e, "ports", err)
		}
	}
	rnge, err := cfg.Ports()
	if err != nil {
		return commandError(e, "ports", err)
	}
	// The short-lived CLI cannot own a reservation after it exits. Its invoking
	// session/worker owns the initial claim; env up hands ownership to the server.
	r, err := s.Claim(t.Path, operands[0], os.Getppid(), rnge)
	if err != nil {
		return commandError(e, "ports", err)
	}
	if *asJSON {
		return writeJSON(e.Stdout, r)
	}
	printPorts(e, r)
	return 0
}
func printPorts(e Env, r ports.Record) {
	fmt.Fprintf(e.Stdout, "%s  %d-%d  pid %d  %s\n", r.Job, r.Start, r.End, r.PID, r.Task)
}

func runEnv(args []string, e Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(e.Stdout, "Usage: grid env up <J> | down <J>")
		return 0
	}
	op := args[0]
	if op != "up" && op != "down" {
		fmt.Fprintf(e.Stderr, "env: unknown command %q\n", op)
		return 2
	}
	fs := newFlags("env "+op, e)
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	if len(operands) != 1 {
		fmt.Fprintf(e.Stderr, "env %s: expected a Job id\n", op)
		return 2
	}
	t, err := environmentTask(e, operands[0])
	if err != nil {
		return commandError(e, "env", err)
	}
	m := devenv.Manager{Ports: ports.Store{GridHome: e.GridHome}, TaskDir: t.Path}
	if op == "down" {
		if err := m.Down(operands[0]); err != nil {
			return commandError(e, "env", err)
		}
		return 0
	}
	cfg, rec, err := devenv.Load(e.GridHome, t.Project)
	if err != nil {
		return commandError(e, "env", err)
	}
	machine := e.Machine
	if machine == "" {
		machine = gridhome.Machine(e.GridHome)
	}
	folders := rec.FoldersOn(machine)
	if len(folders) != 1 {
		return commandError(e, "env", errors.New("project must name exactly one checkout folder for this machine"))
	}
	m.Dir = folders[0]
	// An interrupted health wait must stop the server it started, not orphan it.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := m.Up(ctx, operands[0], cfg)
	if err != nil {
		return commandError(e, "env", err)
	}
	printPorts(e, r)
	return 0
}
