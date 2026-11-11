package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

type planDigestTranscript struct {
	Cwd       string               `json:"cwd"`
	Folder    string               `json:"folder"`
	Task      string               `json:"task"`
	Scenarios []planDigestScenario `json:"scenarios"`
}

type planDigestScenario struct {
	Name          string           `json:"name"`
	RemoveReceipt bool             `json:"remove_receipt,omitempty"`
	Steps         []planDigestStep `json:"steps"`
}

type planDigestStep struct {
	Argv           []string `json:"argv"`
	Exit           int      `json:"exit"`
	StdoutKeys     []string `json:"stdout_keys"`
	ReceiptKeys    []string `json:"receipt_keys"`
	StdoutContains string   `json:"stdout_contains,omitempty"`
	StderrContains string   `json:"stderr_contains,omitempty"`
	AppendPlan     string   `json:"append_plan,omitempty"`
}

func planDigestFixture() string {
	return filepath.Join("..", "..", "testdata", "contract", "plan-digest")
}

// Like golden.Replay, each scenario copies a home into an isolated directory and
// compares observed exits and artifacts. Only keys are recorded here so the app
// can replay the same commands without normalizing clocks, sessions or paths.
func TestPlanDigestRoundTrip(t *testing.T) {
	path := filepath.Join(planDigestFixture(), "transcript.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var transcript planDigestTranscript
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&transcript); err != nil {
		t.Fatal(err)
	}
	update := os.Getenv("UPDATE_PLAN_DIGEST_FIXTURE") == "1"
	for i := range transcript.Scenarios {
		scenario := &transcript.Scenarios[i]
		t.Run(scenario.Name, func(t *testing.T) {
			h := newHarness(t)
			h.person()
			if err := os.CopyFS(h.home, os.DirFS(filepath.Join(planDigestFixture(), "home"))); err != nil {
				t.Fatal(err)
			}
			if !filepath.IsLocal(filepath.FromSlash(transcript.Folder)) {
				t.Fatal("fixture folder must be relative to home")
			}
			folder := filepath.Join(h.home, filepath.FromSlash(transcript.Folder))
			if scenario.RemoveReceipt {
				if err := os.Remove(filepath.Join(folder, "approval.json")); err != nil {
					t.Fatal(err)
				}
			}
			// The receipt of a step is the result of the plan Job, as the readers
			// return it (job, record, who, when, answer, plan_hash, plan_digest,
			// lands_hash, note); nothing is written to approval.json any more.
			jobResult := func() map[string]any {
				out := map[string]any{}
				a, _ := taskstore.ApprovalFromJob(folder)
				if a.Record == "" {
					return out
				}
				data, err := json.Marshal(a)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &out); err != nil {
					t.Fatal(err)
				}
				return out
			}
			tokens := map[string]string{"$home": h.home, "$folder": folder, "$task": transcript.Task}
			expand := func(value string) string {
				if strings.HasPrefix(value, "$") {
					resolved, ok := tokens[value]
					if !ok {
						t.Fatalf("unresolved placeholder %q", value)
					}
					return resolved
				}
				return value
			}
			for i := range scenario.Steps {
				step := &scenario.Steps[i]
				t.Run(fmt.Sprintf("%02d-%s", i+1, strings.Join(step.Argv, "_")), func(t *testing.T) {
					if step.AppendPlan != "" {
						path := filepath.Join(folder, "plan.md")
						data, err := os.ReadFile(path)
						if err != nil {
							t.Fatal(err)
						}
						h.write(path, string(data)+step.AppendPlan)
					}
					args := make([]string, len(step.Argv))
					for i, arg := range step.Argv {
						args[i] = expand(arg)
					}
					before := jobResult()
					code := h.run(expand(transcript.Cwd), args...)
					if args[0] == "approve" && code == 0 {
						// The person has no lease: the holder's next pass accepts the answer.
						holderPass(t, folder)
					}
					// Job render no longer depends on plan approval, so the recorded
					// render exit and stderr (the removed refusal) are not asserted;
					// the receipt checks below still run for those steps.
					render := len(args) > 1 && args[0] == "job" && args[1] == "render"
					if !update && !render && code != step.Exit {
						t.Errorf("exit = %d, want %d; stdout: %s; stderr: %s", code, step.Exit, h.out, h.errb)
					}
					if !strings.Contains(h.out.String(), step.StdoutContains) {
						t.Errorf("stdout missing %q: %s", step.StdoutContains, h.out)
					}
					if !render && !strings.Contains(h.errb.String(), step.StderrContains) {
						t.Errorf("stderr missing %q: %s", step.StderrContains, h.errb)
					}
					stdout := map[string]any{}
					if code == 0 && slices.Contains(args, "--json") {
						if err := json.Unmarshal(h.out.Bytes(), &stdout); err != nil {
							t.Fatalf("stdout JSON: %v", err)
						}
					}
					if args[0] == "plan" && code == 0 {
						for key, length := range map[string]int{"hash": 6, "digest": 64} {
							value, ok := stdout[key].(string)
							if !ok || !regexp.MustCompile(fmt.Sprintf("^[0-9a-f]{%d}$", length)).MatchString(value) {
								t.Fatalf("%s must be %d lowercase hex digits: %v", key, length, stdout[key])
							}
							tokens["$"+key] = value
						}
					}
					receipt := jobResult()
					if (args[0] != "approve" || code != 0) && !reflect.DeepEqual(before, receipt) {
						t.Error("read or rejected approval changed the Job result")
					}
					if len(receipt) > 0 && receipt["plan_hash"] != tokens["$hash"] {
						t.Errorf("receipt plan_hash = %v, want %s", receipt["plan_hash"], tokens["$hash"])
					}
					if args[0] == "approve" && code == 0 {
						if receipt["plan_digest"] != tokens["$digest"] {
							t.Errorf("receipt plan_digest = %v, want captured digest", receipt["plan_digest"])
						}
						// task and what are fields of the event, not of the Job result.
						for _, key := range []string{"plan_hash", "plan_digest", "note"} {
							if slices.Contains(args, "--json") && stdout[key] != receipt[key] {
								t.Errorf("stdout %s = %v, receipt = %v", key, stdout[key], receipt[key])
							}
						}
						if index := slices.Index(args, "--note"); index >= 0 && receipt["note"] != args[index+1] {
							t.Errorf("receipt note = %v, want %q", receipt["note"], args[index+1])
						}
					}
					stdoutKeys, receiptKeys := planDigestKeys(stdout), planDigestKeys(receipt)
					if update {
						step.Exit, step.StdoutKeys, step.ReceiptKeys = code, stdoutKeys, receiptKeys
					} else {
						if !slices.Equal(stdoutKeys, step.StdoutKeys) {
							t.Errorf("stdout keys = %v, want %v", stdoutKeys, step.StdoutKeys)
						}
						if !slices.Equal(receiptKeys, step.ReceiptKeys) {
							t.Errorf("receipt keys = %v, want %v", receiptKeys, step.ReceiptKeys)
						}
					}
				})
			}
		})
	}
	// Reuse the contract golden writer; a failed semantic check never blesses a
	// broken recording, and ordinary runs never write to the fixture directory.
	if update && !t.Failed() {
		contractCommandWrite(t, path, transcript)
	}
}

func planDigestKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func TestPlanDigestFixturePortable(t *testing.T) {
	// Also reject other absolute paths and URL hosts, beyond the required probes.
	private := regexp.MustCompile(`(?i)(/Users/|/home/|/srv/|/tmp/|@|[a-z]:\\|://|(?:^|[\s"'])/[^/])`)
	err := filepath.WalkDir(planDigestFixture(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("fixture must contain only regular files: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if private.Match(data) {
			t.Errorf("fixture contains a private identifier or absolute path: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
