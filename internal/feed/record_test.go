package feed_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"go.yaml.in/yaml/v4"
)

// Entry is the production Record embedding. These extensions also ensure that
// callers can embed either Record or Entry without inheriting a whole-object
// decoder that silently discards their own fields.
type recordExtension struct {
	feed.Record `yaml:",inline"`
	Label       string `json:"label"`
	Count       int    `json:"count"`
}

type recordPointerExtension struct {
	*feed.Record `yaml:",inline"`
	Label        string `json:"label"`
}

type entryExtension struct {
	feed.Entry `yaml:",inline"`
	Label      string `json:"label"`
}

func TestRecordEmbeddingRoundTrip(t *testing.T) {
	r := fixtureRecord(t, "../../contract/fixtures/post/valid/basic.json")
	r.Lease, r.Re, r.Job, r.Wake = strptr("task"), strptr(r.ID), strptr("J-1"), strptr("wake-1")
	attempt := 2
	r.Attempt = &attempt
	r.Gen, r.Fence, r.Seq = 9007199254740993, 9007199254740995, 9007199254740997
	r.After = []string{r.ID}
	e := feed.Entry{Record: r, Arrived: r.At, File: "writer.1.jsonl", End: 1234,
		Verdict: "held", Waiting: []string{r.ID}, Data: true, Reason: "waiting for record"}
	for _, tc := range []struct {
		name string
		want any
	}{
		{"record", r},
		{"entry", e},
		{"record extension", recordExtension{Record: r, Label: "retained", Count: 7}},
		{"record pointer extension", recordPointerExtension{Record: &r, Label: "retained"}},
		{"entry extension", entryExtension{Entry: e, Label: "retained"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, format := range []struct {
				name   string
				encode func(any) ([]byte, error)
				decode func([]byte, any) error
			}{
				{"json", json.Marshal, json.Unmarshal},
				{"yaml", func(v any) ([]byte, error) { return yaml.Dump(v) }, func(b []byte, v any) error { return yaml.Load(b, v) }},
			} {
				t.Run(format.name, func(t *testing.T) {
					data, err := format.encode(tc.want)
					if err != nil {
						t.Fatal(err)
					}
					got := reflect.New(reflect.TypeOf(tc.want))
					if err := format.decode(data, got.Interface()); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got.Elem().Interface(), tc.want) {
						t.Fatalf("round trip lost fields:\ngot  %+v\nwant %+v", got.Elem().Interface(), tc.want)
					}
				})
			}
		})
	}
}

func TestRecordEmbeddingTokenNumbers(t *testing.T) {
	for _, tokens := range []json.Number{"0", "9007199254740993", "18446744073709551615"} {
		t.Run(string(tokens), func(t *testing.T) {
			r := fixtureRecord(t, "../../contract/fixtures/usage-notice/valid/basic.json")
			delete(r.Body, "dollars")
			r.Body["tokens"] = tokens
			want := recordExtension{Record: r, Label: "tokens", Count: 1}
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got recordExtension
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("round trip changed token notice:\ngot  %+v\nwant %+v", got, want)
			}
		})
	}
}

func TestRecordEmbeddingRejectsBadFields(t *testing.T) {
	for _, data := range []string{
		`{"seq":"bad","label":"retained"}`,
		`{"body":[],"label":"retained"}`,
		`{"body":{"text":1e1000},"label":"retained"}`,
		`{"body":{},"count":"bad"}`,
	} {
		var got recordExtension
		if err := json.Unmarshal([]byte(data), &got); err == nil {
			t.Errorf("accepted bad field: %s", data)
		}
	}
}

// Existing contract fixtures are the golden wire records; compare every field
// against the independent contract envelope, including dynamic body Go values,
// then compare the encoded JSON values against the original fixture.
func TestRecordFixtureGolden(t *testing.T) {
	for _, typ := range contract.PostTypes() {
		files, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", typ.Name, "valid", "*.json"))
		if err != nil || len(files) == 0 {
			t.Fatalf("%s fixtures: %v", typ.Name, err)
		}
		for _, file := range files {
			t.Run(typ.Name+"/"+filepath.Base(file), func(t *testing.T) {
				want := fixtureRecord(t, file)
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var got feed.Record
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("decoded values changed:\ngot  %+v\nwant %+v", got, want)
				}
				encoded, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(jsonValue(t, encoded), jsonValue(t, raw)) {
					t.Fatalf("wire values changed: %s", encoded)
				}
			})
		}
	}
}

func fixtureRecord(t *testing.T, file string) feed.Record {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var p contract.Post
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(p.Body, &body); err != nil {
		t.Fatal(err)
	}
	return feed.Record{ID: p.ID, Task: p.Task, Type: p.Type, TypeV: p.TypeV,
		Author: p.Author, Tier: p.Tier, Worker: p.Worker, Lease: p.Lease,
		Gen: p.Gen, Fence: p.Fence, Seq: p.Seq, At: p.At, Re: p.Re,
		After: p.After, Job: p.Job, Attempt: p.Attempt, Wake: p.Wake, Body: body}
}

func jsonValue(t *testing.T, data []byte) any {
	t.Helper()
	var out any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func strptr(s string) *string { return &s }
