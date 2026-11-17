package judge

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

func pjVbState() State {
	return State{Task: "T-1", Jobs: map[string]*contract.Job{}, Bodies: map[string]map[string]any{}, Records: map[string]feed.Record{}}
}

// pjVbJob adds a folded Job and its job record body, as feed state and feed read give them.
func pjVbJob(st *State, id, role string, status contract.Status, attempt int, discoveredFrom, continues string) {
	j := &contract.Job{ID: id, Role: role, Status: status, Attempt: attempt, Title: "Work " + id}
	body := map[string]any{"id": id, "role": role, "title": j.Title, "item": 1, "discovered_from": nil, "continues": nil, "after": []any{},
		"acceptance": []any{map[string]any{"id": "A1", "text": "the work of " + id + " is done", "traces": "item"}}}
	if discoveredFrom != "" {
		j.DiscoveredFrom, body["discovered_from"] = &discoveredFrom, discoveredFrom
	}
	if continues != "" {
		j.Continues, body["continues"] = &continues, continues
	}
	st.Jobs[id] = j
	st.Bodies[id] = body
	st.IDs = append(st.IDs, id)
}

// pjVbRecord adds a counted record; a result or submit is also listed on its Job, as the fold lists it.
func pjVbRecord(st *State, id, typ, job string, attempt int, body map[string]any) {
	r := feed.Record{ID: id, Task: st.Task, Type: typ, Body: body}
	if job != "" {
		r.Job = &job
	}
	if attempt > 0 {
		r.Attempt = &attempt
	}
	st.Records[id] = r
	if j := st.Jobs[job]; j != nil {
		switch typ {
		case "result":
			j.Results = append(j.Results, id)
		case "submit":
			j.Submits = append(j.Submits, id)
		}
	}
}

// pjVbApply folds the posts of a plan into the state the next call reads: a job post adds its Job,
// a control post moves its target.
func pjVbApply(t *testing.T, st *State, posts []Post) {
	t.Helper()
	for _, p := range posts {
		switch p.Type {
		case "job":
			j := &contract.Job{ID: p.Job, Role: str(p.Body["role"]), Status: contract.Posted, Attempt: 1}
			if from := str(p.Body["discovered_from"]); from != "" {
				j.DiscoveredFrom = &from
			}
			if c := str(p.Body["continues"]); c != "" {
				j.Continues = &c
			}
			st.Jobs[p.Job] = j
			st.Bodies[p.Job] = p.Body
			st.IDs = append(st.IDs, p.Job)
		case "control":
			if j := st.Jobs[str(p.Body["target"])]; j != nil {
				j.Status = contract.Status(str(p.Body["status"]))
			}
		}
		r := feed.Record{ID: p.ID(), Task: st.Task, Type: p.Type, Body: p.Body}
		job := p.Job
		r.Job = &job
		if p.Wake != "" {
			wake := p.Wake
			r.Wake = &wake
		}
		st.Records[r.ID] = r
	}
}

// pjVbChain is a builder J-1 rejected by its review J-1.1 and reworked as J-2, whose review J-1.2
// rejected it too.
func pjVbChain() State {
	st := pjVbState()
	pjVbJob(&st, "J-1", "builder", "rejected", 1, "", "")
	pjVbJob(&st, "J-1.1", "reviewer", "accepted", 1, "J-1", "")
	pjVbRecord(&st, "R1", "result", "J-1.1", 1, map[string]any{"outcome": "REJECTED\nMust-fix:\n- row 1"})
	pjVbJob(&st, "J-2", "builder", "submitted", 1, "J-1.1", "J-1")
	pjVbJob(&st, "J-1.2", "grid-reviewer", "submitted", 1, "J-2", "")
	pjVbRecord(&st, "R2", "result", "J-1.2", 1, map[string]any{"outcome": "REJECTED\nstill fails"})
	return st
}

// pjVbSchemaKeys are the property names of the plugin's job record schema, and of one acceptance row.
func pjVbSchemaKeys(t *testing.T) (record, row map[string]bool) {
	t.Helper()
	data, err := os.ReadFile("../../contract/schemas/job-record.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	var acceptance struct {
		Items struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"items"`
	}
	if err := json.Unmarshal(schema.Properties["acceptance"], &acceptance); err != nil {
		t.Fatal(err)
	}
	record, row = map[string]bool{}, map[string]bool{}
	for k := range schema.Properties {
		record[k] = true
	}
	for k := range acceptance.Items.Properties {
		row[k] = true
	}
	if len(record) == 0 || len(row) == 0 {
		t.Fatalf("job-record.json gave %d record keys and %d row keys", len(record), len(row))
	}
	return record, row
}

func pjVbFixture(t *testing.T, id string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = id
	return body
}
