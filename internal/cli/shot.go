package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/shot"
)

const shotUsage = "Usage: grid shot crop <png> [--box x,y,w,h] [--max <px>] [--out <png>] [--json] | diff <png-a> <png-b> [--tile <px>] [--threshold <0-255>] [--json]"

// shotCropJSON is the --json answer of grid shot crop.
type shotCropJSON struct {
	File         string   `json:"file"`
	Box          shot.Box `json:"box"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	SourceWidth  int      `json:"source_width"`
	SourceHeight int      `json:"source_height"`
	Scale        float64  `json:"scale"`
}

func runShot(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, shotUsage)
		fmt.Fprintln(env.Stdout, "crop cuts a box out of a PNG and scales it so its long side is at most 1568 px; open the crop, not the whole shot.")
		fmt.Fprintln(env.Stdout, "diff prints the boxes (x,y,w,h, usable as crop --box) where two PNGs differ, with the percentage of each box that differs.")
		return 0
	}
	op := args[0]
	if op != "crop" && op != "diff" {
		fmt.Fprintf(env.Stderr, "shot: unknown command %q\n", op)
		return 2
	}
	fs := newFlags("shot "+op, env)
	fs.Usage = func() {
		out := usageOut(args, env)
		fmt.Fprintln(out, shotUsage)
		fs.SetOutput(out)
		fs.PrintDefaults()
	}
	asJSON := fs.Bool("json", false, "print JSON")
	var box, outFile string
	maxSide, tile, threshold := shot.MaxSide, shot.DefaultTile, shot.DefaultThreshold
	if op == "crop" {
		fs.StringVar(&box, "box", "", "the part to keep as x,y,w,h in pixels from the top left; default the whole picture")
		fs.IntVar(&maxSide, "max", shot.MaxSide, "longest side of the result in pixels; a larger crop is scaled down")
		fs.StringVar(&outFile, "out", "", "the PNG to write; default <png>-crop-<x>-<y>-<w>-<h>.png beside the input")
	} else {
		fs.IntVar(&tile, "tile", shot.DefaultTile, "differing pixels in tiles of this many pixels that touch join one box")
		fs.IntVar(&threshold, "threshold", shot.DefaultThreshold, "the largest colour channel difference, 0 to 255, that still counts as the same")
	}
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	want := 1
	if op == "diff" {
		want = 2
	}
	if len(operands) != want {
		fmt.Fprintf(env.Stderr, "shot %s: expected %d operands\n", op, want)
		return 2
	}
	path := func(p string) string {
		if filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(env.Cwd, p)
	}
	if op == "diff" {
		a, err := shot.Load(path(operands[0]))
		if err != nil {
			return commandError(env, "shot", err)
		}
		b, err := shot.Load(path(operands[1]))
		if err != nil {
			return commandError(env, "shot", err)
		}
		res, err := shot.Diff(a, b, shot.DiffOptions{Tile: tile, Threshold: threshold})
		if err != nil {
			return commandError(env, "shot", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, res)
		}
		if res.WidthA != res.WidthB || res.HeightA != res.HeightB {
			fmt.Fprintf(env.Stdout, "sizes differ: %dx%d and %dx%d\n", res.WidthA, res.HeightA, res.WidthB, res.HeightB)
		}
		if !res.Differs {
			fmt.Fprintf(env.Stdout, "same %dx%d\n", res.WidthA, res.HeightA)
			return 0
		}
		fmt.Fprintf(env.Stdout, "differs %.2f%% in %d boxes\n", res.Percent, len(res.Boxes))
		for _, r := range res.Boxes {
			fmt.Fprintf(env.Stdout, "%s  %.2f%%\n", r.Box, r.Percent)
		}
		return 0
	}
	var cut *shot.Box
	if box != "" {
		b, err := shot.ParseBox(box)
		if err != nil {
			return commandError(env, "shot", err)
		}
		cut = &b
	}
	img, err := shot.Load(path(operands[0]))
	if err != nil {
		return commandError(env, "shot", err)
	}
	res, err := shot.Crop(img, cut, maxSide)
	if err != nil {
		return commandError(env, "shot", err)
	}
	dest := outFile
	if dest == "" {
		in := path(operands[0])
		dest = fmt.Sprintf("%s-crop-%d-%d-%d-%d.png", strings.TrimSuffix(in, filepath.Ext(in)), res.Box.X, res.Box.Y, res.Box.W, res.Box.H)
	}
	dest = path(dest)
	data, err := shot.Encode(res.Image)
	if err != nil {
		return commandError(env, "shot", err)
	}
	if err := atomicfile.Write(dest, data, 0o644); err != nil {
		return commandError(env, "shot", err)
	}
	b := res.Image.Bounds()
	if *asJSON {
		return writeJSON(env.Stdout, shotCropJSON{File: dest, Box: res.Box, Width: b.Dx(), Height: b.Dy(),
			SourceWidth: res.SourceWidth, SourceHeight: res.SourceHeight, Scale: res.Scale})
	}
	fmt.Fprintf(env.Stdout, "%s  %dx%d  box %s of %dx%d\n", dest, b.Dx(), b.Dy(), res.Box, res.SourceWidth, res.SourceHeight)
	return 0
}
