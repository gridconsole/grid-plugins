package proof

// Screenshots kept next to the posts that link them (docs/research/brain-board.md,
// Q5): a screenshot of a Job's acceptance row is stored at
// <task>/proof/<job>-row<N>.<ext> and the record that describes it goes into the
// body of the result or checkpoint post that adds it. This is the second way to
// store proof; the first (proof/<item>/<id>.<ext> with proof.json) is unchanged.

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// ErrAnimated is an animated gif: it is a video, which stays in the machine folder.
var ErrAnimated = errors.New("an animated gif is a video; add it with kind video, not as a screenshot")

// ShotInput is one screenshot of one acceptance row of a Job.
type ShotInput struct {
	Job  string // J-n
	Row  int    // the acceptance row, 1 or more
	File string // the captured file: png, jpg, webp or a gif of one frame
	By   string
	Note string
}

// Shot is a stored screenshot. Record.File is task-relative (proof/J-4-row2.png);
// Link is the same file as a relative link from a board post (../proof/J-4-row2.png).
type Shot struct {
	Record
	Job  string `json:"job"`
	Row  int    `json:"row"`
	Link string `json:"link"`
}

var shotJob = regexp.MustCompile(`^J-[0-9]+(\.[0-9]+)*$`)

// AddShot stores a screenshot at proof/<job>-row<row>.png or .webp and returns its
// record. A png or webp is stored as captured. A jpg or a gif of one frame is
// decoded and stored as a png with the same decoded pixels, so no JPEG is written
// and the post that links the file never has to change its link. A second shot of
// the same row is stored as <job>-row<row>-2, -3 and so on; an earlier file is
// never replaced, because a post names its sha256.
func (s Store) AddShot(in ShotInput) (Shot, error) {
	var zero Shot
	if !shotJob.MatchString(in.Job) || in.Row < 1 || in.Row > 9999 {
		return zero, errors.New("a screenshot needs a Job like J-4 and a row of 1 or more")
	}
	if strings.TrimSpace(in.By) == "" || strings.ContainsAny(in.By+in.Note, "\r\n\x00") {
		return zero, errors.New("by and note must be single-line values; by is required")
	}
	src, err := ReadEvidence(in.File)
	if err != nil {
		return zero, err
	}
	data, ext, err := shotBytes(src)
	if err != nil {
		return zero, err
	}
	dir, err := filepath.EvalSymlinks(s.TaskDir)
	if err != nil {
		return zero, err
	}
	unlock, err := localdata.LockExisting(dir, ".proof.lock")
	if err != nil {
		return zero, err
	}
	defer unlock()
	folder, err := localdata.Safe(dir, "proof")
	if err != nil {
		return zero, err
	}
	if err = os.MkdirAll(folder, 0755); err != nil {
		return zero, err
	}
	base := fmt.Sprintf("%s-row%d", in.Job, in.Row)
	stem := base
	for n := 2; shotTaken(folder, stem); n++ {
		stem = fmt.Sprintf("%s-%d", base, n)
	}
	name := stem + ext
	path, err := localdata.Safe(dir, "proof", name)
	if err != nil {
		return zero, err
	}
	hash := ""
	if text, err := (plan.Store{TaskDir: dir}).Read(); err == nil {
		hash = plan.Hash(text)
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	if err = atomicfile.Write(path, data, 0644); err != nil {
		return zero, err
	}
	rel := filepath.ToSlash(filepath.Join("proof", name))
	return Shot{
		Record: Record{Kind: "screenshot", By: in.By, At: now.UTC().Format(time.RFC3339Nano), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)),
			Note: in.Note, File: rel, PlanHash: hash},
		Job: in.Job, Row: in.Row, Link: "../" + rel,
	}, nil
}

// shotTaken reports whether proof/<stem>.png or .webp exists.
func shotTaken(folder, stem string) bool {
	for _, ext := range []string{".png", ".webp"} {
		if _, err := os.Lstat(filepath.Join(folder, stem+ext)); err == nil {
			return true
		}
	}
	return false
}

// shotBytes is what is stored for a captured file, and its extension. The format
// is read from the bytes, not from the name the capture tool gave the file.
func shotBytes(src []byte) ([]byte, string, error) {
	if len(src) >= 12 && string(src[:4]) == "RIFF" && string(src[8:12]) == "WEBP" {
		return src, ".webp", nil
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return nil, "", errors.New("screenshot file must be png, jpg, webp or gif")
	}
	var img image.Image
	switch format {
	case "png":
		return src, ".png", nil
	case "jpeg":
		if img, err = jpeg.Decode(bytes.NewReader(src)); err != nil {
			return nil, "", err
		}
	case "gif":
		g, err := gif.DecodeAll(bytes.NewReader(src))
		if err != nil {
			return nil, "", err
		}
		if len(g.Image) != 1 {
			return nil, "", ErrAnimated
		}
		img = g.Image[0]
		if w, h := g.Config.Width, g.Config.Height; w > 0 && h > 0 && g.Image[0].Bounds() != image.Rect(0, 0, w, h) {
			canvas := image.NewNRGBA(image.Rect(0, 0, w, h))
			draw.Draw(canvas, g.Image[0].Bounds(), g.Image[0], g.Image[0].Bounds().Min, draw.Over)
			img = canvas
		}
	default:
		return nil, "", errors.New("screenshot file must be png, jpg, webp or gif")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, "", err
	}
	return out.Bytes(), ".png", nil
}

// PostBody is what a result or checkpoint post that adds screenshots carries
// besides its own keys: the record of each screenshot, and one relative image
// link each, which the post renders as it renders any text. The caller merges
// these keys into the post body.
func PostBody(shots ...Shot) map[string]any {
	records := make([]any, 0, len(shots))
	links := make([]any, 0, len(shots))
	for _, sh := range shots {
		records = append(records, map[string]any{"kind": sh.Kind, "sha256": sh.SHA256, "file": sh.File, "plan_hash": sh.PlanHash, "row": sh.Row})
		links = append(links, fmt.Sprintf("![%s row %d](%s)", sh.Job, sh.Row, sh.Link))
	}
	return map[string]any{"proof": records, "screenshots": links}
}
