package proof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/board/postfile"
)

// pbSpTask is a task folder inside a brain: <root>/projects/demo/tasks/T-7, with
// its board/ next to its proof/.
type pbSpTaskDirs struct {
	task  string
	board postfile.Dir
}

func pbSpTask(t *testing.T) pbSpTaskDirs {
	t.Helper()
	root := t.TempDir()
	task := filepath.Join(root, "brain", "projects", "demo", "tasks", "T-7")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	return pbSpTaskDirs{task: task, board: postfile.Dir{
		Board: filepath.Join(task, "board"), Cache: filepath.Join(root, "machine"), Rel: "projects/demo/tasks/T-7",
	}}
}

// pbSpPicture is a picture whose pixels are all different, so a change shows.
func pbSpPicture() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, 40, 30))
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 6), G: uint8(y * 8), B: uint8((x*y + 7) % 256), A: 255})
		}
	}
	return img
}

func pbSpPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, pbSpPicture()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func pbSpJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, pbSpPicture(), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// pbSpPaletted is a picture of nine colours, as a gif frame holds.
func pbSpPaletted(shift int) *image.Paletted {
	pal := color.Palette{}
	for i := 0; i < 9; i++ {
		pal = append(pal, color.RGBA{R: uint8(i * 28), G: uint8(255 - i*28), B: uint8(i * 11), A: 255})
	}
	img := image.NewPaletted(image.Rect(0, 0, 40, 30), pal)
	for y := 0; y < 30; y++ {
		for x := 0; x < 40; x++ {
			img.SetColorIndex(x, y, uint8((x+y+shift)%9))
		}
	}
	return img
}

func pbSpGIF(t *testing.T, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		g.Image = append(g.Image, pbSpPaletted(i))
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// pbSpWebP is not a picture a decoder here reads: the lane keeps a webp as
// captured, so its bytes only need the RIFF/WEBP header.
func pbSpWebP() []byte {
	return append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 \x18\x00\x00\x00"), bytes.Repeat([]byte{0x9d}, 24)...)
}

func pbSpCapture(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func pbSpAdd(t *testing.T, d pbSpTaskDirs, job string, row int, name string, data []byte) Shot {
	t.Helper()
	sh, err := (Store{TaskDir: d.task}).AddShot(ShotInput{Job: job, Row: row, File: pbSpCapture(t, name, data), By: "builder", Note: "row proof"})
	if err != nil {
		t.Fatalf("AddShot %s: %v", name, err)
	}
	return sh
}

// pbSpPost publishes a result post that carries the shots, and returns the text
// of the post file and its folder.
func pbSpPost(t *testing.T, d pbSpTaskDirs, shots ...Shot) (string, string) {
	t.Helper()
	body := map[string]any{"outcome": "done"}
	for k, v := range PostBody(shots...) {
		body[k] = v
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	l, err := d.board.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer l.Unlock()
	n := 1
	h := postfile.Header{ID: fmt.Sprintf("01JD7Q4ZK0V8M2T6Y1N3B5C9X%d", n), Task: "01JD7P0A1B2C3D4E5F6G7H8J9K", Type: "result", TypeV: 1, Job: "J-4",
		At: "2026-11-17T01:52:10.123Z", Author: postfile.Author{ID: "J-4", Role: "agent"}, By: "01JDMACHINE0000000000000000:grid3",
		After: []string{}, Body: raw}
	_, name, err := l.Publish(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d.board.Board, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b), d.board.Board
}

// pbSpLinked finds the image link of a post and resolves it as a viewer would,
// from the folder of the post.
func pbSpLinked(t *testing.T, post, folder, alt string) string {
	t.Helper()
	open := "![" + alt + "]("
	i := strings.Index(post, open)
	if i < 0 {
		t.Fatalf("the post has no image link %q:\n%s", open, post)
	}
	rest := post[i+len(open):]
	link, _, ok := strings.Cut(rest, ")")
	if !ok || strings.Contains(link, "\n") || filepath.IsAbs(link) || strings.Contains(link, "://") {
		t.Fatalf("the link %q is not a relative path", link)
	}
	return filepath.Join(folder, filepath.FromSlash(link))
}

// V17: a png lands at proof/J-n-rowN.png and a webp at proof/J-n-rowN.webp, each
// linked relatively from its post, and both are stored as captured.
func TestPbSpPNGAndWebPLandNextToThePost(t *testing.T) {
	d := pbSpTask(t)
	cases := []struct {
		name, want string
		data       []byte
		row        int
	}{
		{"shot.png", "proof/J-4-row1.png", pbSpPNG(t), 1},
		{"shot.webp", "proof/J-4-row2.webp", pbSpWebP(), 2},
	}
	for _, c := range cases {
		sh := pbSpAdd(t, d, "J-4", c.row, c.name, c.data)
		if sh.File != c.want || sh.Link != "../"+c.want || sh.Kind != "screenshot" {
			t.Fatalf("%s: stored as %+v, want %s", c.name, sh, c.want)
		}
		got, err := os.ReadFile(filepath.Join(d.task, filepath.FromSlash(c.want)))
		if err != nil || !bytes.Equal(got, c.data) {
			t.Fatalf("%s: the stored file is not the captured bytes (%v)", c.name, err)
		}
		if sh.SHA256 != fmt.Sprintf("%x", sha256.Sum256(c.data)) {
			t.Fatalf("%s: sha256 %s", c.name, sh.SHA256)
		}
		post, folder := pbSpPost(t, d, sh)
		target := pbSpLinked(t, post, folder, fmt.Sprintf("J-4 row %d", c.row))
		if want := filepath.Join(d.task, filepath.FromSlash(c.want)); target != want && filepath.Clean(target) != want {
			t.Fatalf("%s: the link resolves to %s, want %s", c.name, target, want)
		}
		if b, err := os.ReadFile(target); err != nil || !bytes.Equal(b, c.data) {
			t.Fatalf("%s: the link does not lead to the screenshot: %v", c.name, err)
		}
		if !strings.Contains(post, sh.SHA256) || !strings.Contains(post, c.want) {
			t.Fatalf("%s: the post does not carry the proof record:\n%s", c.name, post)
		}
	}
}

// pbSpPixels is the 8-bit colour the standard decoders give each pixel.
func pbSpPixels(t *testing.T, data []byte) (image.Rectangle, []color.NRGBA) {
	t.Helper()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	var out []color.NRGBA
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out = append(out, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
		}
	}
	return b, out
}

// V17: a jpg or a gif of one frame is stored as proof/J-n-rowN.png with the same
// decoded pixels, and the post links the png.
func TestPbSpJPEGAndSingleFrameGIFAreStoredAsPNG(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"shot.jpg", pbSpJPEG(t)},
		{"shot.jpeg", pbSpJPEG(t)},
		{"shot.gif", pbSpGIF(t, 1)},
		{"misnamed.png", pbSpJPEG(t)}, // the bytes decide, not the name
	} {
		t.Run(c.name, func(t *testing.T) {
			d := pbSpTask(t)
			sh := pbSpAdd(t, d, "J-4", 3, c.name, c.data)
			if sh.File != "proof/J-4-row3.png" || sh.Link != "../proof/J-4-row3.png" {
				t.Fatalf("stored as %s (%s)", sh.File, sh.Link)
			}
			stored, err := os.ReadFile(filepath.Join(d.task, "proof", "J-4-row3.png"))
			if err != nil {
				t.Fatal(err)
			}
			if _, format, err := image.DecodeConfig(bytes.NewReader(stored)); err != nil || format != "png" {
				t.Fatalf("the stored file is %q (%v), want png", format, err)
			}
			wantB, want := pbSpPixels(t, c.data)
			gotB, got := pbSpPixels(t, stored)
			if gotB != wantB || len(got) != len(want) {
				t.Fatalf("size %v, want %v", gotB, wantB)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("pixel %d is %v, want %v", i, got[i], want[i])
				}
			}
			if sh.SHA256 != fmt.Sprintf("%x", sha256.Sum256(stored)) {
				t.Fatal("the record names the sha256 of the original, not of the stored png")
			}
			post, folder := pbSpPost(t, d, sh)
			target := pbSpLinked(t, post, folder, "J-4 row 3")
			if b, err := os.ReadFile(target); err != nil || !bytes.Equal(b, stored) || !strings.HasSuffix(target, ".png") {
				t.Fatalf("the post does not link the png: %s %v", target, err)
			}
			entries, _ := os.ReadDir(filepath.Join(d.task, "proof"))
			for _, e := range entries {
				if ext := strings.ToLower(filepath.Ext(e.Name())); ext == ".jpg" || ext == ".jpeg" || ext == ".gif" {
					t.Fatalf("%s was stored beside the png", e.Name())
				}
			}
		})
	}
}

// A second shot of a row never replaces the first, whose sha256 a post names.
func TestPbSpSecondShotOfARowKeepsTheFirst(t *testing.T) {
	d := pbSpTask(t)
	first := pbSpAdd(t, d, "J-4", 2, "a.png", pbSpPNG(t))
	second := pbSpAdd(t, d, "J-4", 2, "b.jpg", pbSpJPEG(t))
	third := pbSpAdd(t, d, "J-4", 2, "c.webp", pbSpWebP())
	if first.File != "proof/J-4-row2.png" || second.File != "proof/J-4-row2-2.png" || third.File != "proof/J-4-row2-3.webp" {
		t.Fatalf("names %s %s %s", first.File, second.File, third.File)
	}
	if b, _ := os.ReadFile(filepath.Join(d.task, "proof", "J-4-row2.png")); !bytes.Equal(b, pbSpPNG(t)) {
		t.Fatal("the first shot was replaced")
	}
}

// An animated gif is a video: it is not stored as a screenshot, and nothing is written.
func TestPbSpAnimatedGIFIsNotAScreenshot(t *testing.T) {
	d := pbSpTask(t)
	_, err := (Store{TaskDir: d.task}).AddShot(ShotInput{Job: "J-4", Row: 1, File: pbSpCapture(t, "a.gif", pbSpGIF(t, 3)), By: "builder"})
	if !errors.Is(err, ErrAnimated) {
		t.Fatalf("err = %v, want ErrAnimated", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(d.task, "proof")); len(entries) != 0 {
		t.Fatalf("%d files were written", len(entries))
	}
}

func TestPbSpShotRefusesBadInput(t *testing.T) {
	d := pbSpTask(t)
	for _, c := range []struct {
		name string
		in   ShotInput
	}{
		{"job", ShotInput{Job: "../J-4", Row: 1, By: "b"}},
		{"row", ShotInput{Job: "J-4", Row: 0, By: "b"}},
		{"by", ShotInput{Job: "J-4", Row: 1}},
		{"text", ShotInput{Job: "J-4", Row: 1, By: "b", File: pbSpCapture(t, "x.png", []byte("not a picture"))}},
	} {
		if _, err := (Store{TaskDir: d.task}).AddShot(c.in); err == nil {
			t.Errorf("%s: no error", c.name)
		}
	}
}
