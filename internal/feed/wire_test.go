package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// TestWrittenRecordsMatchContract rebuilds every valid contract fixture of every
// post type through the feed's own record builder and validates the result
// against the contract schema, so the wire envelope cannot drift from main's.
func TestWrittenRecordsMatchContract(t *testing.T) {
	types := contract.PostTypes()
	if len(types) == 0 {
		t.Fatal("no post types in the contract")
	}
	for _, typ := range types {
		files, _ := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", typ.Name, "valid", "*.json"))
		if len(files) == 0 {
			t.Fatalf("%s: no valid fixtures", typ.Name)
		}
		for _, file := range files {
			t.Run(typ.Name+"/"+filepath.Base(file), func(t *testing.T) {
				raw, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var fx contract.Post
				if err := json.Unmarshal(raw, &fx); err != nil {
					t.Fatal(err)
				}
				var body map[string]any
				if err := json.Unmarshal(fx.Body, &body); err != nil {
					t.Fatal(err)
				}
				in := Input{ID: fx.ID, Type: fx.Type, Body: body, Re: value(fx.Re), Job: value(fx.Job), Wake: value(fx.Wake), After: fx.After}
				if fx.Attempt != nil {
					in.Attempt = *fx.Attempt
				}
				w := Writer{Author: fx.Author, Worker: fx.Worker, Lease: value(fx.Lease), Gen: fx.Gen, Fence: fx.Fence}
				s := Store{TaskDir: t.TempDir(), TaskID: fx.Task}
				r, err := s.record(in, w)
				if err != nil {
					t.Fatal(err)
				}
				r.Seq = fx.Seq
				if r.Tier != fx.Tier || r.Author != fx.Author || r.TypeV != typ.Version {
					t.Fatalf("stamped tier %q author %+v type_v %d, fixture %q %+v %d", r.Tier, r.Author, r.TypeV, fx.Tier, fx.Author, typ.Version)
				}
				data, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if err := contract.Validate(typ.Name, data); err != nil {
					t.Fatalf("%s\n%s", err, data)
				}
			})
		}
	}
}

// contractBody returns the body of the contract's basic valid fixture of typ
// with over applied, so tests post bodies the contract schema accepts.
func contractBody(t *testing.T, typ string, over map[string]any) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", typ, "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Body map[string]any `json:"body"`
	}
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatal(err)
	}
	for k, v := range over {
		fx.Body[k] = v
	}
	return fx.Body
}
