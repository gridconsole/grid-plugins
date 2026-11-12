package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The fakes below act like the app's real programs for the one thing grid open decides:
// the argument list. Each writes its argv, one argument per line, to
// $FAKE_MARKERS/<name>.argv (through a rename, so a reader never sees half of it).
// grid-server exits 2 unless arg 0 is serve and the argv holds --home $GRID_HOME, --who
// local and --role owner (app cmd/grid-server/main.go); only then does it write app-url.
// grid-worker exits 2 unless arg 0 is start (app cmd/grid-worker/main.go).
const waOpenServerBody = `printf '%s\n' "$@" > "$FAKE_MARKERS/grid-server.argv.tmp"
mv "$FAKE_MARKERS/grid-server.argv.tmp" "$FAKE_MARKERS/grid-server.argv"
if [ "$1" != serve ]; then echo "usage: grid-server serve ..." >&2; exit 2; fi
home= who= role= prev=
for a in "$@"; do
	case $prev in
	--home) home=$a ;;
	--who) who=$a ;;
	--role) role=$a ;;
	esac
	prev=$a
done
if [ "$home" != "$GRID_HOME" ] || [ "$who" != local ] || [ "$role" != owner ]; then
	echo "grid-server: --home, --who and --role are required" >&2
	exit 2
fi
printf 'http://%s\n' "$FAKE_ADDR" > "$GRID_HOME/app-url"
`

const waOpenWorkerBody = `printf '%s\n' "$@" > "$FAKE_MARKERS/grid-worker.argv.tmp"
mv "$FAKE_MARKERS/grid-worker.argv.tmp" "$FAKE_MARKERS/grid-worker.argv"
if [ "$1" != start ]; then echo "usage: grid-worker start|status|stop ..." >&2; exit 2; fi
`

// waOpenRun runs grid open T-01 --json on a home with no app running, with both fakes
// replaced by the argv-checking ones, and returns the fixture, the exit code, the url
// the JSON holds and the addresses handed to the opener.
func waOpenRun(t *testing.T) (f *startFixture, code int, url string, opened []string) {
	t.Helper()
	f = newStartFixture(t)
	f.program("grid-server", waOpenServerBody)
	f.program("grid-worker", waOpenWorkerBody)
	f.env.AppStart.Wait = 5 * time.Second
	f.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	code = f.run(f.home, "open", "T-01", "--json")
	var got struct {
		URL string `json:"url"`
	}
	if f.out.Len() > 0 {
		if err := json.Unmarshal(f.out.Bytes(), &got); err != nil {
			t.Fatalf("stdout %q is not the JSON of grid open: %v", f.out, err)
		}
	}
	return f, code, got.URL, opened
}

// waOpenArgv reads the argv a fake program wrote, one argument per line. The program
// is started detached, so the file may land a moment after grid open returns.
func waOpenArgv(t *testing.T, f *startFixture, name string) []string {
	t.Helper()
	path := filepath.Join(f.markers, name+".argv")
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		raw, err := os.ReadFile(path)
		if err == nil {
			return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s wrote no argv file: %v", name, err)
		}
	}
}

func TestWaOpenStartsServerWithServe(t *testing.T) {
	f, code, url, opened := waOpenRun(t)
	argv := waOpenArgv(t, f, "grid-server")
	want := []string{"serve", "--addr", "127.0.0.1:0", "--home", f.gridHome, "--who", "local", "--role", "owner"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("grid-server argv %q, want %q", argv, want)
	}
	if code != 0 {
		t.Fatalf("open exit %d, want 0: %s", code, f.errb)
	}
	if url != f.want() {
		t.Errorf("json url %q, want %q", url, f.want())
	}
	if len(opened) != 1 || opened[0] != f.want() {
		t.Errorf("opened %v, want %s", opened, f.want())
	}
}

func TestWaOpenStartsWorkerWithStart(t *testing.T) {
	f, code, _, _ := waOpenRun(t)
	t.Logf("open exit %d", code)
	argv := waOpenArgv(t, f, "grid-worker")
	want := []string{"start", "-home", f.gridHome}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("grid-worker argv %q, want %q", argv, want)
	}
	const line = "grid open: started grid-worker and grid-server."
	if !strings.Contains(f.errb.String(), line) {
		t.Errorf("stderr %q does not hold %q", f.errb, line)
	}
}
