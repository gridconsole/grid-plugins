package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// Explicit server configuration is the permission authority. Desktop bridge
// discovery is intentionally absent; offline or unanswered requests fall back
// to the terminal. No redirects can forward tool input to a different endpoint.
func forwardPermission(o Options, in hookio.Input) hookio.Decision {
	base := o.Getenv("GRID_URL")
	if base == "" {
		return hookio.Decision{}
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		o.log("permission", errors.New("invalid GRID_URL"))
		return hookio.Decision{}
	}
	ip := net.ParseIP(u.Hostname())
	loopback := u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		o.log("permission", errors.New("permission server needs HTTPS or loopback HTTP"))
		return hookio.Decision{}
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/hooks/permission"
	timeout := 8 * time.Second
	if ms, err := strconv.Atoi(o.Getenv("GRID_PERMISSION_TIMEOUT_MS")); err == nil && ms > 0 && ms < 8000 {
		timeout = time.Duration(ms) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	b, err := json.Marshal(in)
	if err != nil {
		o.log("permission", err)
		return hookio.Decision{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(b))
	if err != nil {
		o.log("permission", errors.New("cannot create permission request"))
		return hookio.Decision{}
	}
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		o.log("permission", errors.New("permission server unavailable; using terminal"))
		return hookio.Decision{}
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		o.log("permission", errors.New("permission server did not answer; using terminal"))
		return hookio.Decision{}
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 64*1024+1))
	if err != nil || len(body) > 64*1024 {
		o.log("permission", errors.New("invalid permission response"))
		return hookio.Decision{}
	}
	var answer struct {
		Behavior     string         `json:"behavior"`
		Message      string         `json:"message"`
		UpdatedInput map[string]any `json:"updatedInput"`
		Interrupt    bool           `json:"interrupt"`
	}
	if json.Unmarshal(body, &answer) != nil {
		o.log("permission", errors.New("invalid permission response"))
		return hookio.Decision{}
	}
	if answer.Behavior != "allow" && answer.Behavior != "deny" {
		return hookio.Decision{}
	}
	d := hookio.Decision{Event: "PermissionRequest", Behavior: answer.Behavior}
	if answer.Behavior == "deny" {
		d.Reason = answer.Message
		d.Interrupt = answer.Interrupt
	} else if answer.UpdatedInput != nil {
		in.ToolInput = answer.UpdatedInput
		check := evaluateGuards(o, in, guardEnv(o))
		if check.Behavior == "deny" {
			check.Event = "PermissionRequest"
			return check
		}
		d.UpdatedInput = answer.UpdatedInput
	}
	return d
}
