package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestR23DeliveryDocNamesNoRemovedCommand: docs/delivery.md must not send a
// reader to the removed preview command, the removed approve flag or the
// receipt files a908a013 dropped (U-chk-plugin-rev-cli row 5).
func TestR23DeliveryDocNamesNoRemovedCommand(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "delivery.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, bad := range []string{"grid preview", "--gate", "gate: ship", "gate: preview", "preview approved"} {
		if strings.Contains(text, bad) {
			t.Errorf("docs/delivery.md still names %q", bad)
		}
	}
	// "preview command" may appear only as "no preview command".
	flat := strings.Join(strings.Fields(text), " ")
	for i := 0; ; {
		j := strings.Index(flat[i:], "preview command")
		if j < 0 {
			break
		}
		j += i
		if !strings.HasSuffix(flat[:j], "no ") {
			t.Errorf("docs/delivery.md says %q without a leading \"no\"", flat[max(0, j-12):j+len("preview command")])
		}
		i = j + 1
	}
	// The receipt files may be named only as no longer written or read.
	for _, f := range []string{"approval-ship.json", "approval-preview.json"} {
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, f) && !strings.Contains(text, "no longer written or read") {
				t.Errorf("docs/delivery.md names %s as live", f)
			}
		}
	}
}

// TestR23DeliveringTopicsMatchDelivery: the topic files say how a topic is
// reached (no hook) and that a waiting proposal does not block delivery
// (U-chk-plugin-rev-skills rows 7 and 8).
func TestR23DeliveringTopicsMatchDelivery(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("..", "..", "topics", name, "TOPIC.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	for _, name := range []string{"delivering", "reviewing"} {
		if strings.Contains(read(name), "Read by the hooks") {
			t.Errorf("topics/%s/TOPIC.md says a hook reads it; topics/README.md says none does", name)
		}
	}
	d := read("delivering")
	for _, bad := range []string{"block delivery", "no brain proposal waits", "refuses (waiting proposals"} {
		if strings.Contains(d, bad) {
			t.Errorf("topics/delivering/TOPIC.md still says %q", bad)
		}
	}
}
