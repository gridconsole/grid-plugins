package lint

import (
	"bytes"
	"encoding/json"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// IsJSON reports whether a lint input is a Job record rather than a markdown
// brief: a .json file name, or content whose first non-space byte is '{'.
func IsJSON(name string, text []byte) bool {
	if len(name) >= 5 && name[len(name)-5:] == ".json" {
		return true
	}
	t := bytes.TrimLeft(bytes.TrimPrefix(text, []byte{0xEF, 0xBB, 0xBF}), " \t\r\n")
	return len(t) > 0 && t[0] == '{'
}

// CheckJSON checks a Job record. It runs the job-record schema first and
// reports its error with the field path; a record that passes is decoded and
// goes through the same rules as a markdown brief. The list is never nil.
func CheckJSON(data []byte, o Options) []Finding {
	c := checker{findings: []Finding{}}
	if err := contract.Validate("job-record", data); err != nil {
		c.add("job.record", "", err.Error(), "Fix the field the error names so the record matches the job-record schema.")
		return c.finish()
	}
	var j job.Job
	if err := json.Unmarshal(data, &j); err != nil {
		c.add("job.type", "", err.Error(), "Use the field types in the Job contract.")
		return c.finish()
	}
	j.Defaults()
	c.validate(j, o)
	return c.finish()
}
