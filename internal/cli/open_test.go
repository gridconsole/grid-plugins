package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const openStable = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

// openFixture makes one task in a migrated home (id ULID, display T-01) or a legacy one (id T-01).
// The link id is the ULID in both shapes: the page links by it and shows T-01.
func openFixture(t *testing.T, migrated bool) (h *harness, project string, id string) {
	t.Helper()
	h, project, id, _ = openFixtureAt(t, migrated)
	return
}

func openFixtureAt(t *testing.T, migrated bool) (h *harness, project, id, folder string) {
	t.Helper()
	h = newHarness(t)
	t.Setenv("GRID_APP_URL", "")
	fixture := taskFixture(t, h)
	ids := "id: T-01\nulid: " + openStable + "\n"
	if migrated {
		ids = "id: " + openStable + "\ndisplay: T-01\n"
	} else if err := os.Remove(filepath.Join(h.gridHome, "VERSION")); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(fixture.Path, "task.md"), "---\n"+ids+"title: A title\nproject: "+fixture.Project+"\nsize: small\nworkflow: \"\"\nstate: queued\ncreated: '2026-01-02T03:04:05.000Z'\ncontract: \"1\"\n---\n")
	return h, fixture.Project, openStable, fixture.Path
}

func TestOpenOpensTaskPageInBrowser(t *testing.T) {
	for _, migrated := range []bool{true, false} {
		name := "legacy"
		if migrated {
			name = "migrated"
		}
		t.Run(name, func(t *testing.T) {
			h, project, id, folder := openFixtureAt(t, migrated)
			h.write(filepath.Join(h.gridHome, AppURLFile), "http://127.0.0.1:8484/\n")
			var opened []string
			h.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
			want := "http://127.0.0.1:8484/#/t/" + project + "/" + id + "/overview"
			// By display id from anywhere, and from inside the task folder with no operand.
			if code := h.run(h.home, "open", "T-01"); code != 0 {
				t.Fatalf("open T-01: %d %s", code, h.errb)
			}
			if len(opened) != 1 || opened[0] != want {
				t.Fatalf("opened %v, want %s", opened, want)
			}
			if !strings.Contains(h.out.String(), want) {
				t.Errorf("stdout %q lacks the URL", h.out)
			}
			if code := h.run(folder, "open"); code != 0 {
				t.Fatalf("open in folder: %d %s", code, h.errb)
			}
			if code := h.run(h.home, "open", openStable); code != 0 {
				t.Fatalf("open by ULID: %d %s", code, h.errb)
			}
			if len(opened) != 3 || opened[1] != want || opened[2] != want {
				t.Fatalf("opened %v, want %s three times", opened, want)
			}
		})
	}
}

func TestOpenWithNoBrowserPrintsURL(t *testing.T) {
	h, project, id := openFixture(t, true)
	t.Setenv("GRID_APP_URL", "https://grid.example.test")
	h.env.OpenURL = func(string) error { return errors.New("no display") }
	if code := h.run(h.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, h.errb)
	}
	want := "https://grid.example.test/#/t/" + project + "/" + id + "/overview"
	if strings.TrimSpace(h.out.String()) != want {
		t.Fatalf("stdout %q, want %s", h.out, want)
	}
}

func TestOpenJSONPrintsURL(t *testing.T) {
	h, project, id := openFixture(t, true)
	t.Setenv("GRID_APP_URL", "http://localhost:9000")
	h.env.OpenURL = func(string) error { return nil }
	if code := h.run(h.home, "open", "T-01", "--json"); code != 0 {
		t.Fatalf("open: %d %s", code, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("json: %q %v", h.out, err)
	}
	if len(got) != 1 || got["url"] != "http://localhost:9000/#/t/"+project+"/"+id+"/overview" {
		t.Fatalf("json: %v", got)
	}
}

func TestOpenWithoutAppAddressPrintsPath(t *testing.T) {
	h, project, id := openFixture(t, true)
	h.env.OpenURL = func(u string) error { t.Fatalf("opened %s with no app address", u); return nil }
	if code := h.run(h.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, h.errb)
	}
	if want := "#/t/" + project + "/" + id + "/overview"; strings.TrimSpace(h.out.String()) != want {
		t.Fatalf("stdout %q, want %s", h.out, want)
	}
	if !strings.Contains(h.errb.String(), "Start the app") {
		t.Errorf("stderr %q does not say to start the app", h.errb)
	}
}

func TestOpenUnknownTaskFails(t *testing.T) {
	h, _, _ := openFixture(t, true)
	h.env.OpenURL = func(u string) error { t.Fatalf("opened %s", u); return nil }
	if code := h.run(h.home, "open", "T-99"); code == 0 {
		t.Fatalf("open T-99 exited 0: %s", h.out)
	}
}
