package jobcmd

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// These follow the CLI's existing status convention. There are no shared named
// usage or not-found constants on the landing base.
const (
	traceExitOK       = 0
	traceExitFailed   = 1
	traceExitNotFound = 1
	traceExitUsage    = 2
	traceUsage        = "Usage: grid job trace <pointer> [--sha256 <digest>] [--project <name-or-id>] [--json]\n"
)

// TraceRow preserves one matching source from a filed Job. Filed is the bundle
// directory's modification time; it is not proof that the Job was executed.
type TraceRow struct {
	Project string             `json:"project"`
	Task    string             `json:"task"`
	Job     string             `json:"job"`
	Pointer string             `json:"pointer"`
	SHA256  string             `json:"sha256"`
	Updated string             `json:"updated"`
	Tier    contract.TrustTier `json:"tier"`
	Filed   string             `json:"filed"`
}

// TraceProblem identifies unreadable evidence without exposing filesystem paths.
type TraceProblem struct {
	Task string `json:"task"`
	Job  string `json:"job"`
	Text string `json:"text"`
}

type TraceResult struct {
	Rows     []TraceRow     `json:"rows"`
	Problems []TraceProblem `json:"problems"`
}

var errTraceProjectNotFound = errors.New("project not found")
var traceRangeSuffix = regexp.MustCompile(`:[0-9]+-[0-9]+$`)

// tracePointer mirrors the anchor and line-range separation in Files.Resolve.
// It deliberately does not clean or otherwise normalize the path part.
func tracePointer(pointer string) string {
	path, _, _ := strings.Cut(pointer, "#")
	return traceRangeSuffix.ReplaceAllString(path, "")
}

func traceCheckQuery(pointer, digest string) error {
	path := tracePointer(pointer)
	if strings.TrimSpace(path) == "" || strings.ContainsAny(pointer, "\r\n\x00") {
		return errors.New("trace requires a nonempty pointer on one line")
	}
	if strings.HasPrefix(path, "brain:") {
		if _, _, ok := job.ParseBrain(path); !ok {
			return errors.New("invalid brain pointer; use brain:<home|project>/<name>")
		}
	}
	if digest != "" {
		b, err := hex.DecodeString(digest)
		if err != nil || len(b) != 32 {
			return errors.New("--sha256 requires a 64-character hexadecimal digest")
		}
	}
	return nil
}

func traceMatch(pointer, source string) bool {
	pointer, source = tracePointer(pointer), tracePointer(source)
	level, name, ok := job.ParseBrain(pointer)
	if !ok {
		return pointer == source
	}
	otherLevel, otherName, otherOK := job.ParseBrain(source)
	return otherOK && level == otherLevel && name == otherName
}

// tracePath adds a no-symlink rule to Within, including links whose targets are
// inside the root. The root itself may have a platform alias (such as on macOS).
func tracePath(root, rel string) (string, error) {
	path, err := job.Within(root, rel)
	if err != nil {
		return "", err
	}
	current := root
	for _, part := range strings.Split(filepath.FromSlash(rel), string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("symbolic link refused")
		}
	}
	return path, nil
}

func traceProject(home, filter string, dirs []string) (string, error) {
	if filter == "" {
		return "", nil
	}
	for _, dir := range dirs {
		if name := filepath.Base(filepath.Dir(dir)); name == filter {
			return name, nil
		}
	}
	records, err := project.Load(home)
	if err != nil {
		return "", errors.New("cannot read project records")
	}
	found := ""
	for _, rec := range records {
		if rec.ID == filter {
			if found != "" {
				return "", errors.New("project id is ambiguous")
			}
			found = rec.Name
		}
	}
	if found == "" {
		return "", errTraceProjectNotFound
	}
	return found, nil
}

// Trace reads filed inputs across Personal and all projects without consulting
// live brain entries or changing Jobs. Problems do not discard other evidence.
func Trace(home, pointer, digest, projectFilter string) (TraceResult, error) {
	result := TraceResult{Rows: []TraceRow{}, Problems: []TraceProblem{}}
	if err := traceCheckQuery(pointer, digest); err != nil {
		return result, err
	}
	if home == "" {
		return result, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	dirs, err := gridhome.TasksDirs(home)
	if err != nil {
		return result, errors.New("cannot list project task folders")
	}
	selected, err := traceProject(home, projectFilter, dirs)
	if err != nil {
		return result, err
	}
	for _, dir := range dirs {
		proj := filepath.Base(filepath.Dir(dir))
		if selected != "" && proj != selected {
			continue
		}
		rel, err := filepath.Rel(home, dir)
		if err == nil {
			dir, err = tracePath(home, rel)
		}
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			result.Problems = append(result.Problems, TraceProblem{Text: fmt.Sprintf("project %q: task folder is unreadable or unsafe", proj)})
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			result.Problems = append(result.Problems, TraceProblem{Text: fmt.Sprintf("project %q: cannot list tasks", proj)})
			continue
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), "T-") {
				continue
			}
			path, err := tracePath(dir, entry.Name())
			if err != nil || !entry.IsDir() {
				result.Problems = append(result.Problems, TraceProblem{Task: entry.Name(), Text: "task folder is unreadable or unsafe"})
				continue
			}
			// task.Read owns legacy task front matter; validate its two reads first.
			safe := true
			for _, name := range []string{"task.md", "intent.md"} {
				p, err := tracePath(path, name)
				if err != nil {
					safe = false
					break
				}
				info, err := os.Stat(p)
				if err != nil || !info.Mode().IsRegular() {
					safe = false
					break
				}
			}
			if !safe {
				result.Problems = append(result.Problems, TraceProblem{Task: entry.Name(), Text: "task metadata is unreadable or unsafe"})
				continue
			}
			record, err := task.Read(path)
			if err != nil || record.Project != proj {
				result.Problems = append(result.Problems, TraceProblem{Task: entry.Name(), Text: "invalid task metadata or mismatched project"})
				continue
			}
			traceScanTask(&result, record, pointer, digest)
		}
	}
	sort.SliceStable(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Task != b.Task {
			return a.Task < b.Task
		}
		return a.Job < b.Job
	})
	return result, nil
}

func traceScanTask(result *TraceResult, record task.Task, pointer, digest string) {
	problem := func(id, text string) {
		result.Problems = append(result.Problems, TraceProblem{Task: record.ID, Job: id, Text: text})
	}
	if _, err := os.Lstat(filepath.Join(record.Path, "jobs")); os.IsNotExist(err) {
		return
	}
	dir, err := tracePath(record.Path, "jobs")
	if err != nil {
		problem("", "jobs folder is unreadable or unsafe")
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		problem("", "cannot list Job bundles")
		return
	}
	for _, entry := range entries {
		// Temporary render directories have not been published as immutable Jobs.
		if strings.HasPrefix(entry.Name(), ".render-") {
			continue
		}
		id := entry.Name()
		bundle, err := tracePath(record.Path, filepath.Join("jobs", id))
		if err != nil || !entry.IsDir() {
			problem(id, "Job bundle is unreadable or unsafe")
			continue
		}
		info, err := os.Stat(bundle)
		if err != nil {
			problem(id, "cannot read bundle modification time")
			continue
		}
		path, err := tracePath(record.Path, filepath.Join("jobs", id, "inputs.json"))
		if err != nil {
			problem(id, "inputs.json is unreadable or unsafe")
			continue
		}
		inputInfo, err := os.Stat(path)
		if err != nil || !inputInfo.Mode().IsRegular() {
			problem(id, "inputs.json must be a regular file")
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			problem(id, "cannot read inputs.json")
			continue
		}
		var inputs *job.Inputs
		if err := json.Unmarshal(data, &inputs); err != nil || inputs == nil {
			problem(id, "invalid inputs.json")
			continue
		}
		for _, source := range inputs.Sources {
			if !traceMatch(pointer, source.Pointer) || (digest != "" && digest != source.Digest) {
				continue
			}
			result.Rows = append(result.Rows, TraceRow{Project: record.Project, Task: record.ID, Job: id, Pointer: source.Pointer, SHA256: source.Digest, Updated: source.Updated, Tier: source.Tier, Filed: contract.FormatTime(info.ModTime())})
		}
	}
}

func runTrace(args []string, e Env) int {
	asJSON := false
	for _, a := range args {
		if a == "--json" {
			asJSON = true
		}
	}
	fail := func(err error, code int) int {
		if asJSON {
			return traceOutput(e, TraceResult{Rows: []TraceRow{}, Problems: []TraceProblem{{Text: err.Error()}}}, true, code)
		}
		if _, writeErr := fmt.Fprintf(e.Stderr, "job trace: %s\n", err); writeErr != nil {
			return traceExitFailed
		}
		return code
	}
	var pointer, digest, filter string
	positions := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
		case "--help", "-h":
			if _, err := io.WriteString(e.Stdout, traceUsage); err != nil {
				return traceExitFailed
			}
			return traceExitOK
		case "--sha256", "--project":
			if i+1 == len(args) || strings.HasPrefix(args[i+1], "--") || strings.TrimSpace(args[i+1]) == "" {
				return fail(fmt.Errorf("%s needs a value", a), traceExitUsage)
			}
			i++
			if a == "--sha256" {
				digest = args[i]
			} else {
				filter = args[i]
			}
		default:
			if strings.HasPrefix(a, "-") {
				return fail(fmt.Errorf("unknown flag %s", a), traceExitUsage)
			}
			positions++
			if positions > 1 {
				return fail(errors.New("trace requires exactly one pointer"), traceExitUsage)
			}
			pointer = a
		}
	}
	if err := traceCheckQuery(pointer, digest); err != nil {
		return fail(err, traceExitUsage)
	}
	result, err := Trace(e.GridHome, pointer, digest, filter)
	if errors.Is(err, errTraceProjectNotFound) {
		return fail(err, traceExitNotFound)
	}
	if err != nil {
		return fail(err, traceExitFailed)
	}
	return traceOutput(e, result, asJSON, traceExitOK)
}

func traceOutput(e Env, result TraceResult, asJSON bool, code int) int {
	if asJSON {
		if err := json.NewEncoder(e.Stdout).Encode(result); err != nil {
			return traceExitFailed
		}
		return code
	}
	var b strings.Builder
	for _, row := range result.Rows {
		fmt.Fprintf(&b, "%q %q %q %q sha256=%q updated=%q tier=%q filed=%q\n", row.Project, row.Task, row.Job, row.Pointer, row.SHA256, row.Updated, row.Tier, row.Filed)
	}
	for _, problem := range result.Problems {
		fmt.Fprintf(&b, "problem: %q %q: %s\n", problem.Task, problem.Job, problem.Text)
	}
	if _, err := io.WriteString(e.Stdout, b.String()); err != nil {
		return traceExitFailed
	}
	return code
}
