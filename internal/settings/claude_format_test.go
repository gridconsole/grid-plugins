package settings

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestClaudeFormattingAndLiteralOperators(t *testing.T) {
	f := newClaudeFixture(t)
	const rule = "Bash(grid a && grid b < input > output)"
	f.write(f.path, `{"permissions":{"allow":["`+rule+`"]}}`)
	f.o.Yes = true
	for _, remove := range []bool{false, true} {
		f.o.Remove = remove
		f.wantCode(0)
		data := f.read(f.path)
		if !bytes.Contains(data, []byte(rule)) {
			t.Fatalf("escaped rule: %s", data)
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, bytes.TrimSpace(data), "", "  "); err != nil {
			t.Fatal(err)
		}
		pretty.WriteByte('\n')
		if !bytes.Equal(data, pretty.Bytes()) {
			t.Fatalf("settings are not indented: %s", data)
		}
	}
}
