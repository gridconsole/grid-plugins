// Package env starts and stops the local dev server for a browser Job.
package env

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/ports"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// Config is the single local environments entry in project.md. Command is a
// shell command; {port} and PORT both name the first port of the reserved block.
// Timeout is a Go duration string. PortRange is an inclusive [first,last] pair.
type Config struct {
	Command   string `json:"command"`
	HealthURL string `json:"health_url"`
	Timeout   string `json:"timeout"`
	PortRange []int  `json:"port_range"`
}

// Load first reads the project identity through internal/project. Its Record
// intentionally has no environment fields, so the shared frontmatter parser
// reads this extension without changing the project package or writing a file.
func Load(home, name string) (Config, project.Record, error) {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return Config{}, project.Record{}, errors.New("a project is required")
	}
	data, err := os.ReadFile(filepath.Join(home, "projects", name, "project.md"))
	if err != nil {
		return Config{}, project.Record{}, err
	}
	rec, err := project.Parse(name, string(data))
	if err != nil {
		return Config{}, rec, err
	}
	doc, err := frontmatter.Parse(string(data))
	if err != nil {
		return Config{}, rec, err
	}
	var cfg Config
	if raw, ok := doc.Data["environments"]; ok {
		if err := frontmatter.Into(raw, &cfg); err != nil {
			return cfg, rec, fmt.Errorf("environments: %w", err)
		}
	}
	_, err = cfg.Ports()
	return cfg, rec, err
}
func (c Config) Ports() (ports.Range, error) {
	r := ports.Range{}.Defaults()
	if c.PortRange != nil {
		if len(c.PortRange) != 2 {
			return r, errors.New("environments.port_range must be [first, last]")
		}
		r = ports.Range{Start: c.PortRange[0], End: c.PortRange[1]}
	}
	return r, r.Validate()
}
func (c Config) Validate() (time.Duration, error) {
	if strings.TrimSpace(c.Command) == "" {
		return 0, errors.New("environments.command is required")
	}
	if !strings.Contains(c.HealthURL, "{port}") {
		return 0, errors.New("environments.health_url must contain {port}")
	}
	u, err := url.Parse(strings.ReplaceAll(c.HealthURL, "{port}", "41000"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return 0, errors.New("environments.health_url must be an HTTP or HTTPS URL without credentials")
	}
	timeout := 30 * time.Second
	if c.Timeout != "" {
		timeout, err = time.ParseDuration(c.Timeout)
		if err != nil || timeout <= 0 {
			return 0, errors.New("environments.timeout must be a positive duration such as 30s")
		}
	}
	if _, err := c.Ports(); err != nil {
		return 0, err
	}
	return timeout, nil
}
