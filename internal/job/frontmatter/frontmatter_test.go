package frontmatter

import "testing"

func TestSupportedSubset(t *testing.T) {
	text := "---\nitems:\n  - id: J-1\n    title: 'Quoted: title'\n    links:\n      - {to: brain:project/a, rel: uses, why: \"A reason, with punctuation\"}\n    after: [J-2, gate:Q-1]\n    intent: {owner: intent.md#L1, item: plan.md#step-1}\n---\nBody\n"
	d, e := Parse(text)
	if e != nil {
		t.Fatal(e)
	}
	if d.Body != "Body\n" || d.Lines["items"] != 2 {
		t.Fatalf("%+v", d)
	}
	a, ok := d.Data["items"].([]any)
	if !ok || len(a) != 1 {
		t.Fatal(d.Data)
	}
	m := a[0].(map[string]any)
	if m["title"] != "Quoted: title" || len(m["links"].([]any)) != 1 {
		t.Fatal(m)
	}
}

func TestDatesStayText(t *testing.T) {
	d, e := Parse("---\nupdated: 2026-01-01\nitems: [1, 2]\nok: true\n---\n")
	if e != nil {
		t.Fatal(e)
	}
	if d.Data["updated"] != "2026-01-01" || d.Data["ok"] != true || len(d.Data["items"].([]any)) != 2 {
		t.Fatalf("%#v", d.Data)
	}
	var out struct {
		Updated string `json:"updated"`
		Items   []int  `json:"items"`
	}
	if e = Into(d.Data, &out); e != nil || out.Updated != "2026-01-01" || len(out.Items) != 2 {
		t.Fatalf("%+v %v", out, e)
	}
}

func TestLinesAndBodyAreFileLines(t *testing.T) {
	d, e := Parse("---\nid: J-1\n\nrole: builder\n---\nBody\n")
	if e != nil {
		t.Fatal(e)
	}
	if d.Lines["id"] != 2 || d.Lines["role"] != 4 || d.BodyLine != 6 {
		t.Fatalf("%+v", d)
	}
}

func TestEmptyFrontmatter(t *testing.T) {
	d, e := Parse("---\n---\nBody\n")
	if e != nil || len(d.Data) != 0 || d.Body != "Body\n" {
		t.Fatalf("%+v %v", d, e)
	}
}

func TestRejectMalformed(t *testing.T) {
	for _, body := range []string{"id: J-1\nid: J-2", "x: [a, b", "x: {a: one, a: two}", "x: *alias", "x:\n\t- one", "- a\n- b", "? [a]\n: b"} {
		if _, e := Parse("---\n" + body + "\n---\n"); e == nil {
			t.Errorf("accepted %q", body)
		}
	}
	for _, text := range []string{"No block", "---\nid: J-1"} {
		if _, e := Parse(text); e == nil {
			t.Errorf("accepted %q", text)
		}
	}
}
