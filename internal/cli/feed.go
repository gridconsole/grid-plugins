package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"sort"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runFeed(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid feed post|read|state|types|handover|fold [flags]")
		return 2
	}
	verb := args[0]
	if verb == "--help" || verb == "-h" {
		fmt.Fprintln(env.Stdout, "usage: grid feed post|read|state|types|handover|fold [flags]")
		return 0
	}
	switch verb {
	case "post", "read", "state", "types", "handover", "fold":
	default:
		fmt.Fprintf(env.Stderr, "grid feed: unknown command %q\n", verb)
		return 2
	}
	fs := newFlags("feed "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	task := fs.String("task", "", "task folder path or display id")
	project := fs.String("project", "", "project name or id")
	var kind, body, re, job, id, since, after, wake, lease, epoch, reason string
	var attempt int
	write := verb == "post" || verb == "handover"
	if verb == "post" {
		fs.StringVar(&kind, "type", "", "registered post type")
	}
	if write {
		defaultBody := ""
		if verb == "handover" {
			defaultBody = "{}"
		}
		fs.StringVar(&body, "body", defaultBody, "JSON object, or - for stdin")
		fs.StringVar(&id, "id", "", "ULID reused on retries")
		fs.StringVar(&re, "re", "", "record ULID this follows")
		fs.StringVar(&job, "job", "", "Job id")
		fs.IntVar(&attempt, "attempt", 0, "Job attempt")
		fs.StringVar(&after, "after", "", "comma-separated record ULIDs")
		fs.StringVar(&wake, "wake", os.Getenv("GRID_WAKE"), "wake event ULID/n")
		fs.StringVar(&lease, "lease", os.Getenv("GRID_LEASE"), "task or Job lease")
		fs.StringVar(&epoch, "lease-epoch", os.Getenv("GRID_LEASE_EPOCH"), "lease generation.fence")
	}
	if verb == "handover" {
		fs.StringVar(&reason, "reason", "", "reason for the handover")
	}
	if verb == "read" {
		fs.StringVar(&kind, "type", "", "filter by post type")
		fs.StringVar(&job, "job", "", "filter by Job id")
		fs.StringVar(&since, "since", "", "exclusive display cursor (record ULID or RFC3339 time)")
	}
	rest := args[1:]
	// Preserve the old CLI's task-first shorthand.
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		*task = rest[0]
		rest = rest[1:]
	}
	if code, stop := parse(fs, rest, env); stop {
		return code
	}
	output := func(v any) int {
		if *asJSON {
			return writeJSON(env.Stdout, v)
		}
		if err := feedPlain(env.Stdout, v); err != nil {
			return 1
		}
		return 0
	}
	if verb == "types" {
		return output(struct {
			Types []feed.Type `json:"types"`
		}{feed.Types()})
	}
	fail := func(err error) int {
		fmt.Fprintf(env.Stderr, "grid feed: %v\n", err)
		if errors.Is(err, feed.ErrStaleLease) {
			return 3
		}
		if errors.Is(err, feed.ErrDiskFull) {
			return 4
		}
		return 1
	}
	if env.GridHome == "" {
		return fail(fmt.Errorf("cannot find the Grid home; set GRID_HOME"))
	}
	format, err := gridhome.ReadFormat(env.GridHome)
	if err != nil {
		return fail(err)
	}
	if format > gridhome.FormatVersion {
		return fail(fmt.Errorf("This data is format %d; this grid reads up to %d. Update grid.", format, gridhome.FormatVersion))
	}
	if format < gridhome.FormatVersion {
		return fail(fmt.Errorf("This data is format %d; this grid reads %d. Run: grid migrate", format, gridhome.FormatVersion))
	}
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *project})
	if err != nil {
		return fail(err)
	}
	if res.TaskPath == "" {
		return fail(fmt.Errorf("no task selected; use --task"))
	}
	taskID, err := feed.TaskID(res.TaskPath)
	if err != nil {
		return fail(err)
	}
	store := feed.Store{TaskDir: res.TaskPath, TaskID: taskID}
	switch verb {
	case "read":
		result, err := store.Read(feed.Filter{Since: since, Type: kind, Job: job})
		if err != nil {
			return fail(err)
		}
		return output(result)
	case "state", "fold":
		state, err := store.SaveState()
		if err != nil {
			return fail(err)
		}
		return output(state)
	}
	if body == "-" {
		data, err := io.ReadAll(env.Stdin)
		if err != nil {
			return fail(err)
		}
		body = string(data)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(body), &obj); err != nil || obj == nil {
		return fail(fmt.Errorf("--body must be a JSON object"))
	}
	if reason != "" {
		obj["reason"] = reason
	}
	writer, err := feedWriter(env, lease, epoch)
	if err != nil {
		return fail(err)
	}
	in := feed.Input{ID: id, Type: kind, Body: obj, Re: re, Job: job, Attempt: attempt, Wake: wake}
	if after != "" {
		for _, a := range strings.Split(after, ",") {
			in.After = append(in.After, strings.TrimSpace(a))
		}
	}
	var record feed.Record
	if verb == "handover" {
		record, err = store.Handover(in, writer)
	} else {
		record, err = store.Post(in, writer)
	}
	if err != nil {
		return fail(err)
	}
	return output(struct {
		Post feed.Record `json:"post"`
		Tier string      `json:"tier"`
		Data bool        `json:"data"`
	}{record, string(record.Tier), record.Data()})
}

func feedWriter(env Env, lease, epoch string) (feed.Writer, error) {
	member := os.Getenv("GRID_MEMBER")
	if member == "" {
		cmd := exec.Command("git", "config", "user.email")
		cmd.Dir = env.Cwd
		if out, err := cmd.Output(); err == nil {
			member = strings.TrimSpace(string(out))
		}
	}
	if member == "" {
		if u, err := user.Current(); err == nil {
			member = u.Username
		}
	}
	if member == "" {
		return feed.Writer{}, fmt.Errorf("cannot find member identity; set GRID_MEMBER")
	}
	role := os.Getenv("GRID_ROLE")
	if role == "" {
		role = "member"
	}
	machine, err := feed.MachineID(env.GridHome)
	if err != nil {
		return feed.Writer{}, err
	}
	writer := feed.Writer{Author: feed.Author{ID: member, Role: role}, Worker: member + "@" + machine, Lease: lease}
	if epoch != "" {
		parts := strings.Split(epoch, ".")
		if len(parts) != 2 {
			return writer, fmt.Errorf("lease epoch must be gen.fence")
		}
		writer.Gen, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return writer, fmt.Errorf("invalid lease generation")
		}
		writer.Fence, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return writer, fmt.Errorf("invalid lease fence")
		}
	}
	return writer, nil
}

// feedPlain keeps JSON field names and quotes strings, so embedded newlines and
// terminal controls are data. An untrusted record's body has the fixed data fence.
func feedPlain(w io.Writer, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	var walk func(string, any) error
	walk = func(prefix string, v any) error {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for key := range x {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) == 0 {
				_, err := fmt.Fprintf(w, "%s: {}\n", prefix)
				return err
			}
			for _, key := range keys {
				label := key
				if prefix != "" {
					label = prefix + "." + key
				}
				if key == "body" {
					recRaw, _ := json.Marshal(x)
					var r feed.Record
					_ = json.Unmarshal(recRaw, &r)
					if r.Data() {
						if _, err := fmt.Fprintf(w, "%s:\n%s\n", label, feed.MarshalData(r)); err != nil {
							return err
						}
						continue
					}
					b, _ := json.Marshal(x[key])
					if _, err := fmt.Fprintf(w, "%s: %s\n", label, b); err != nil {
						return err
					}
					continue
				}
				if err := walk(label, x[key]); err != nil {
					return err
				}
			}
		case []any:
			if len(x) == 0 {
				_, err := fmt.Fprintf(w, "%s: []\n", prefix)
				return err
			}
			for i, item := range x {
				if err := walk(fmt.Sprintf("%s[%d]", prefix, i), item); err != nil {
					return err
				}
			}
		default:
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			if err := enc.Encode(v); err != nil {
				return err
			}
			_, err := fmt.Fprintf(w, "%s: %s", prefix, b.String())
			return err
		}
		return nil
	}
	return walk("", value)
}
