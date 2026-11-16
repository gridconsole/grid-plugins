package cli

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shotFixture writes a w by h PNG that is white, with each box painted red.
func shotFixture(t *testing.T, dir, name string, w, h int, boxes ...image.Rectangle) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i++ {
		img.Pix[i] = 255
	}
	for _, b := range boxes {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				img.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
			}
		}
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func shotSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

func TestShotCropWritesBoxScaledDown(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	src := shotFixture(t, dir, "page.png", 3840, 2400)
	out := filepath.Join(dir, "part.png")
	if code := h.run(dir, "shot", "crop", src, "--box", "0,0,3840,2400", "--out", out, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &raw); err != nil {
		t.Fatalf("not JSON: %v: %s", err, h.out.String())
	}
	for _, key := range []string{"file", "box", "width", "height", "source_width", "source_height", "scale"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("--json has no %q: %s", key, h.out.String())
		}
	}
	if raw["width"] != float64(1568) || raw["height"] != float64(980) || raw["source_width"] != float64(3840) {
		t.Errorf("json = %v", raw)
	}
	if w, hgt := shotSize(t, out); w != 1568 || hgt != 980 {
		t.Fatalf("file is %dx%d, want 1568x980", w, hgt)
	}
}

func TestShotCropDefaultNameAndText(t *testing.T) {
	h := newHarness(t)
	dir := t.TempDir()
	shotFixture(t, dir, "page.png", 400, 300)
	if code := h.run(dir, "shot", "crop", "page.png", "--box", "10,20,100,50"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	want := filepath.Join(dir, "page-crop-10-20-100-50.png")
	if w, hgt := shotSize(t, want); w != 100 || hgt != 50 {
		t.Fatalf("%s is %dx%d, want 100x50", want, w, hgt)
	}
	if line := h.out.String(); !strings.Contains(line, want) || !strings.Contains(line, "100x50") || !strings.Contains(line, "10,20,100,50") {
		t.Fatalf("text line = %q", line)
	}
}

func TestShotCropRefusals(t *testing.T) {
	dir := t.TempDir()
	src := shotFixture(t, dir, "page.png", 100, 100)
	notPNG := filepath.Join(dir, "x.png")
	if err := os.WriteFile(notPNG, []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		args []string
		code int
		want string
	}{
		"outside":    {[]string{"shot", "crop", src, "--box", "90,0,50,50"}, 1, "outside the 100x100 picture"},
		"bad box":    {[]string{"shot", "crop", src, "--box", "1,2,3"}, 1, "want x,y,w,h"},
		"not a png":  {[]string{"shot", "crop", notPNG}, 1, "not a PNG"},
		"no operand": {[]string{"shot", "crop"}, 2, "expected 1 operands"},
		"bad flag":   {[]string{"shot", "crop", src, "--nope"}, 2, "not defined"},
		"bad verb":   {[]string{"shot", "bogus"}, 2, "unknown command"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			code := h.run(dir, tc.args...)
			if code != tc.code || !strings.Contains(h.errb.String(), tc.want) {
				t.Fatalf("exit %d, stderr %q; want exit %d with %q", code, h.errb.String(), tc.code, tc.want)
			}
		})
	}
}

func TestShotDiffTextAndJSON(t *testing.T) {
	dir := t.TempDir()
	a := shotFixture(t, dir, "a.png", 200, 100)
	b := shotFixture(t, dir, "b.png", 200, 100, image.Rect(10, 10, 30, 20), image.Rect(150, 60, 160, 70))

	h := newHarness(t)
	if code := h.run(dir, "shot", "diff", a, b); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	lines := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	want := []string{"differs 1.50% in 2 boxes", "10,10,20,10  100.00%", "150,60,10,10  100.00%"}
	if strings.Join(lines, "|") != strings.Join(want, "|") {
		t.Fatalf("text = %q, want %q", lines, want)
	}

	h = newHarness(t)
	if code := h.run(dir, "shot", "diff", a, b, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb.String())
	}
	var res struct {
		WidthA  int     `json:"width_a"`
		HeightB int     `json:"height_b"`
		Differs bool    `json:"differs"`
		Percent float64 `json:"percent"`
		Boxes   []struct {
			X, Y, W, H int
			Pixels     int
			Percent    float64
		} `json:"boxes"`
	}
	var raw struct {
		Boxes []map[string]any `json:"boxes"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &raw); err != nil {
		t.Fatalf("not JSON: %v: %s", err, h.out.String())
	}
	if len(raw.Boxes) != 2 {
		t.Fatalf("boxes = %v", raw.Boxes)
	}
	for _, key := range []string{"x", "y", "w", "h", "pixels", "percent"} {
		if _, ok := raw.Boxes[0][key]; !ok {
			t.Errorf("box has no %q: %v", key, raw.Boxes[0])
		}
	}
	if raw.Boxes[0]["x"] != float64(10) || raw.Boxes[0]["w"] != float64(20) || raw.Boxes[1]["y"] != float64(60) {
		t.Errorf("boxes = %v", raw.Boxes)
	}
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Differs || res.Percent != 1.5 || res.WidthA != 200 || res.HeightB != 100 {
		t.Fatalf("res = %+v", res)
	}
}

func TestShotDiffSameAndEmptyBoxesAreAList(t *testing.T) {
	dir := t.TempDir()
	a := shotFixture(t, dir, "a.png", 50, 50)
	b := shotFixture(t, dir, "b.png", 50, 50)
	h := newHarness(t)
	if code := h.run(dir, "shot", "diff", a, b); code != 0 || strings.TrimSpace(h.out.String()) != "same 50x50" {
		t.Fatalf("exit %d, out %q", code, h.out.String())
	}
	h = newHarness(t)
	if code := h.run(dir, "shot", "diff", a, b, "--json"); code != 0 || !strings.Contains(h.out.String(), `"boxes":[]`) {
		t.Fatalf("exit %d, out %q", code, h.out.String())
	}
}

func TestShotHelpNamesBothVerbs(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "shot", "--help"); code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"crop", "diff", "--box", "1568"} {
		if !strings.Contains(h.out.String(), want) {
			t.Errorf("help lacks %q: %s", want, h.out.String())
		}
	}
}
