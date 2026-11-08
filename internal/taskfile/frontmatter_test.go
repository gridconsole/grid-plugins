package taskfile

import (
	"encoding/json"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	for _, tt := range []struct{ name, text, want string }{
		{"missing", "body", `null`},
		{"scalars", "---\r\na: 'yes' # note\r\nb: false\r\nn: 3\r\n---\r\nbody", `{"data":{"a":"yes","b":"false","n":"3"},"body":"body","odd":false}`},
		{"lists", "---\nworkflow:\n  - {step: plan, kind: approve}\nx: [one, [two, three], {}, '']\n---\n", `{"data":{"workflow":[{"kind":"approve","step":"plan"}],"x":["one",["two","three"],{}]},"body":"","odd":false}`},
		{"odd nested", "---\nx: {constructor: yes}\n---", `{"data":{"x":{"constructor":"yes"}},"body":"","odd":true}`},
		{"duplicate", "---\nx: a\nx: b\n---\nbody\n", `{"data":{"x":"b"},"body":"body\n","odd":false}`},
		{"quoted comma is split", "---\nx: ['a,b']\n---", `{"data":{"x":["'a","b'"]},"body":"","odd":false}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(ParseFrontmatter(tt.text))
			if string(b) != tt.want {
				t.Fatalf("got %s; want %s", b, tt.want)
			}
		})
	}
}
func TestHistoryLine(t *testing.T) {
	for _, tt := range []struct{ detail, via, want string }{
		{"", "", "- time · event · by person"},
		{"detail", "app", "- time · event · detail · by person · via app"},
		{" raw\ntext ", "", "- time · event ·  raw\ntext  · by person"},
	} {
		if got := HistoryLine("time", "event", tt.detail, "person", tt.via); got != tt.want {
			t.Errorf("got %q; want %q", got, tt.want)
		}
	}
}
