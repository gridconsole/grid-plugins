// Package shot crops and compares screenshots so a check reads numbers first and
// opens only a small part of a picture. It uses the standard image packages only.
package shot

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
)

// MaxSide is the longest side, in pixels, of a picture a model reads in full:
// a larger one is scaled down by the reader before the model sees it.
const MaxSide = 1568

// Box is a rectangle in pixels: X and Y are the top left corner.
type Box struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// String is the x,y,w,h form that ParseBox and grid shot crop --box read.
func (b Box) String() string { return fmt.Sprintf("%d,%d,%d,%d", b.X, b.Y, b.W, b.H) }

// ParseBox reads "x,y,w,h": four whole numbers, x and y from the top left corner.
func ParseBox(s string) (Box, error) {
	parts := strings.Split(strings.TrimSpace(s), ",")
	if len(parts) != 4 {
		return Box{}, fmt.Errorf("box %q: want x,y,w,h", s)
	}
	var n [4]int
	for i, p := range parts {
		v, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return Box{}, fmt.Errorf("box %q: %q is not a whole number", s, p)
		}
		n[i] = v
	}
	b := Box{X: n[0], Y: n[1], W: n[2], H: n[3]}
	if b.X < 0 || b.Y < 0 || b.W <= 0 || b.H <= 0 {
		return Box{}, fmt.Errorf("box %q: x and y must be 0 or more, w and h more than 0", s)
	}
	return b, nil
}

// Load reads a PNG file.
func Load(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%s: not a PNG: %w", path, err)
	}
	return img, nil
}

// CropResult is what Crop made and where it came from.
type CropResult struct {
	Image        image.Image
	Box          Box // the part of the source that was cut
	SourceWidth  int
	SourceHeight int
	Scale        float64 // output side over source side; 1 when not scaled
}

// Crop cuts box out of img (the whole picture when box is nil) and scales the cut
// down so its long side is at most max (MaxSide when max is 0). It never scales up.
// A box that reaches outside the picture is an error that names the picture size.
func Crop(img image.Image, box *Box, max int) (CropResult, error) {
	if max < 0 {
		return CropResult{}, errors.New("max must be 0 or more")
	}
	if max == 0 {
		max = MaxSide
	}
	b := img.Bounds()
	res := CropResult{SourceWidth: b.Dx(), SourceHeight: b.Dy(), Scale: 1}
	cut := Box{X: 0, Y: 0, W: b.Dx(), H: b.Dy()}
	if box != nil {
		cut = *box
	}
	if cut.W <= 0 || cut.H <= 0 || cut.X < 0 || cut.Y < 0 || cut.X+cut.W > b.Dx() || cut.Y+cut.H > b.Dy() {
		return CropResult{}, fmt.Errorf("box %s is outside the %dx%d picture", cut, b.Dx(), b.Dy())
	}
	res.Box = cut
	src := toNRGBA(img)
	sub := src.SubImage(image.Rect(cut.X, cut.Y, cut.X+cut.W, cut.Y+cut.H)).(*image.NRGBA)
	long := cut.W
	if cut.H > long {
		long = cut.H
	}
	if long <= max {
		res.Image = sub
		return res, nil
	}
	res.Scale = float64(max) / float64(long)
	w := int(math.Round(float64(cut.W) * res.Scale))
	h := int(math.Round(float64(cut.H) * res.Scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	res.Image = shrink(sub, w, h)
	return res, nil
}

// Encode writes img as PNG bytes.
func Encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// toNRGBA gives img as a picture whose bounds start at 0,0 with straight alpha.
func toNRGBA(img image.Image) *image.NRGBA {
	b := img.Bounds()
	if n, ok := img.(*image.NRGBA); ok && b.Min == (image.Point{}) {
		return n
	}
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out
}

// shrink scales src down to w by h with an area average, so thin lines and small
// text stay visible instead of dropping out as they do with nearest-neighbour.
func shrink(src *image.NRGBA, w, h int) *image.NRGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, b, a, n float64
			for yy := y0; yy < y1; yy++ {
				row := src.Pix[(sb.Min.Y+yy-src.Rect.Min.Y)*src.Stride:]
				for xx := x0; xx < x1; xx++ {
					i := (sb.Min.X + xx - src.Rect.Min.X) * 4
					al := float64(row[i+3])
					// Weight colour by alpha so a clear pixel does not tint its neighbours.
					r += float64(row[i]) * al
					g += float64(row[i+1]) * al
					b += float64(row[i+2]) * al
					a += al
					n++
				}
			}
			o := out.PixOffset(x, y)
			if a > 0 {
				out.Pix[o] = uint8(math.Round(r / a))
				out.Pix[o+1] = uint8(math.Round(g / a))
				out.Pix[o+2] = uint8(math.Round(b / a))
			}
			out.Pix[o+3] = uint8(math.Round(a / n))
		}
	}
	return out
}

// Region is one part of the picture that differs.
type Region struct {
	Box
	Pixels  int     `json:"pixels"`  // differing pixels inside the box
	Percent float64 `json:"percent"` // differing pixels as a percentage of the box area
}

// DiffResult compares two pictures.
type DiffResult struct {
	WidthA  int `json:"width_a"`
	HeightA int `json:"height_a"`
	WidthB  int `json:"width_b"`
	HeightB int `json:"height_b"`
	// Differs is true when any pixel differs or the sizes are not the same.
	Differs bool `json:"differs"`
	// Percent is the differing pixels as a percentage of the larger picture's area.
	Percent float64  `json:"percent"`
	Boxes   []Region `json:"boxes"`
}

// DiffOptions tunes Diff.
type DiffOptions struct {
	// Tile is the side, in pixels, of the squares that are joined into boxes: two
	// differing pixels in tiles that touch land in one box. 0 means DefaultTile.
	Tile int
	// Threshold is the largest difference of one colour channel, 0 to 255, that
	// still counts as the same pixel; 0 compares exactly. DefaultThreshold hides
	// compression noise.
	Threshold int
}

// Default Tile and Threshold.
const (
	DefaultTile      = 16
	DefaultThreshold = 8
)

// Diff compares a and b. Where the sizes differ it compares the larger area and
// counts every pixel that only one picture has as different.
func Diff(a, b image.Image, opt DiffOptions) (DiffResult, error) {
	if opt.Tile == 0 {
		opt.Tile = DefaultTile
	}
	if opt.Tile < 1 {
		return DiffResult{}, errors.New("tile must be 1 or more")
	}
	if opt.Threshold < 0 || opt.Threshold > 255 {
		return DiffResult{}, errors.New("threshold must be 0 to 255")
	}
	pa, pb := toNRGBA(a), toNRGBA(b)
	wa, ha := pa.Bounds().Dx(), pa.Bounds().Dy()
	wb, hb := pb.Bounds().Dx(), pb.Bounds().Dy()
	w, h := max(wa, wb), max(ha, hb)
	res := DiffResult{WidthA: wa, HeightA: ha, WidthB: wb, HeightB: hb, Boxes: []Region{}}
	if w == 0 || h == 0 {
		return res, nil
	}
	tw, th := (w+opt.Tile-1)/opt.Tile, (h+opt.Tile-1)/opt.Tile
	// Per tile: how many pixels differ, and the bounds of those pixels.
	type tile struct {
		n              int
		x0, y0, x1, y1 int
		parent         int
	}
	tiles := make([]tile, tw*th)
	for i := range tiles {
		tiles[i] = tile{x0: math.MaxInt, y0: math.MaxInt, x1: -1, y1: -1, parent: i}
	}
	total := 0
	thr := opt.Threshold
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			inA, inB := x < wa && y < ha, x < wb && y < hb
			diff := false
			switch {
			case inA && inB:
				ia, ib := y*pa.Stride+x*4, y*pb.Stride+x*4
				for c := 0; c < 4; c++ {
					d := int(pa.Pix[ia+c]) - int(pb.Pix[ib+c])
					if d > thr || -d > thr {
						diff = true
						break
					}
				}
			default:
				diff = true
			}
			if !diff {
				continue
			}
			total++
			t := &tiles[(y/opt.Tile)*tw+x/opt.Tile]
			t.n++
			t.x0, t.y0 = min(t.x0, x), min(t.y0, y)
			t.x1, t.y1 = max(t.x1, x), max(t.y1, y)
		}
	}
	res.Differs = total > 0
	res.Percent = pct(total, w*h)
	// Join tiles that touch, sides and corners, into one box each.
	var find func(i int) int
	find = func(i int) int {
		for tiles[i].parent != i {
			tiles[i].parent = tiles[tiles[i].parent].parent
			i = tiles[i].parent
		}
		return i
	}
	for ty := 0; ty < th; ty++ {
		for tx := 0; tx < tw; tx++ {
			i := ty*tw + tx
			if tiles[i].n == 0 {
				continue
			}
			for _, d := range [][2]int{{1, 0}, {0, 1}, {1, 1}, {-1, 1}} {
				nx, ny := tx+d[0], ty+d[1]
				if nx < 0 || nx >= tw || ny >= th {
					continue
				}
				j := ny*tw + nx
				if tiles[j].n > 0 {
					tiles[find(j)].parent = find(i)
				}
			}
		}
	}
	groups := map[int]*tile{}
	for i := range tiles {
		if tiles[i].n == 0 {
			continue
		}
		r := find(i)
		g, ok := groups[r]
		if !ok {
			g = &tile{x0: math.MaxInt, y0: math.MaxInt, x1: -1, y1: -1}
			groups[r] = g
		}
		g.n += tiles[i].n
		g.x0, g.y0 = min(g.x0, tiles[i].x0), min(g.y0, tiles[i].y0)
		g.x1, g.y1 = max(g.x1, tiles[i].x1), max(g.y1, tiles[i].y1)
	}
	for _, g := range groups {
		r := Region{Box: Box{X: g.x0, Y: g.y0, W: g.x1 - g.x0 + 1, H: g.y1 - g.y0 + 1}, Pixels: g.n}
		r.Percent = pct(g.n, r.W*r.H)
		res.Boxes = append(res.Boxes, r)
	}
	sort.Slice(res.Boxes, func(i, j int) bool {
		bi, bj := res.Boxes[i], res.Boxes[j]
		if bi.Y != bj.Y {
			return bi.Y < bj.Y
		}
		return bi.X < bj.X
	})
	return res, nil
}

// pct is part over whole as a percentage with two decimals.
func pct(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return math.Round(float64(part)*10000/float64(whole)) / 100
}
