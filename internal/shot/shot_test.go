package shot

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// solid is a w by h picture in one colour.
func solid(w, h int, c color.NRGBA) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

// paint fills the box with c.
func paint(img *image.NRGBA, b Box, c color.NRGBA) {
	for y := b.Y; y < b.Y+b.H; y++ {
		for x := b.X; x < b.X+b.W; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

// def is the option set the CLI uses when no flag is given.
var def = DiffOptions{Tile: DefaultTile, Threshold: DefaultThreshold}

var (
	white = color.NRGBA{255, 255, 255, 255}
	red   = color.NRGBA{255, 0, 0, 255}
	blue  = color.NRGBA{0, 0, 255, 255}
)

func TestParseBox(t *testing.T) {
	b, err := ParseBox("10, 20,30,40")
	if err != nil || b != (Box{10, 20, 30, 40}) {
		t.Fatalf("ParseBox = %v, %v", b, err)
	}
	for _, bad := range []string{"", "1,2,3", "1,2,3,4,5", "a,2,3,4", "-1,0,5,5", "0,0,0,5", "0,0,5,-2"} {
		if _, err := ParseBox(bad); err == nil {
			t.Errorf("ParseBox(%q) accepted", bad)
		}
	}
}

func TestCropBoxKeepsSizeAndPixels(t *testing.T) {
	src := solid(100, 80, white)
	paint(src, Box{20, 10, 30, 20}, red)
	res, err := Crop(src, &Box{20, 10, 30, 20}, 0)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Image.Bounds()
	if b.Dx() != 30 || b.Dy() != 20 {
		t.Fatalf("crop size = %dx%d, want 30x20", b.Dx(), b.Dy())
	}
	if res.Box != (Box{20, 10, 30, 20}) || res.Scale != 1 || res.SourceWidth != 100 || res.SourceHeight != 80 {
		t.Fatalf("result = %+v", res)
	}
	if got := color.NRGBAModel.Convert(res.Image.At(b.Min.X, b.Min.Y)); got != red {
		t.Fatalf("first pixel = %v, want red", got)
	}
}

func TestCropScalesLongSideToMax(t *testing.T) {
	src := solid(3840, 2400, white)
	res, err := Crop(src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	b := res.Image.Bounds()
	if b.Dx() != 1568 || b.Dy() != 980 {
		t.Fatalf("size = %dx%d, want 1568x980", b.Dx(), b.Dy())
	}
	// A tall crop is limited on its height.
	res, err = Crop(src, &Box{0, 0, 400, 2400}, 600)
	if err != nil {
		t.Fatal(err)
	}
	b = res.Image.Bounds()
	if b.Dx() != 100 || b.Dy() != 600 {
		t.Fatalf("tall size = %dx%d, want 100x600", b.Dx(), b.Dy())
	}
	if res.Scale != 0.25 {
		t.Fatalf("scale = %v, want 0.25", res.Scale)
	}
}

func TestCropNeverScalesUp(t *testing.T) {
	res, err := Crop(solid(50, 40, white), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if b := res.Image.Bounds(); b.Dx() != 50 || b.Dy() != 40 {
		t.Fatalf("size = %dx%d, want 50x40", b.Dx(), b.Dy())
	}
}

func TestCropScaleAveragesColour(t *testing.T) {
	// Left half red, right half blue: halving keeps both colours, the seam is not invented.
	src := solid(200, 100, red)
	paint(src, Box{100, 0, 100, 100}, blue)
	res, err := Crop(src, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Image.(*image.NRGBA)
	if out.NRGBAAt(10, 10) != red || out.NRGBAAt(90, 10) != blue {
		t.Fatalf("scaled colours = %v %v", out.NRGBAAt(10, 10), out.NRGBAAt(90, 10))
	}
}

func TestCropBoxOutsidePictureIsRefused(t *testing.T) {
	_, err := Crop(solid(100, 80, white), &Box{90, 0, 20, 10}, 0)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("100x80")) {
		t.Fatalf("err = %v, want the picture size named", err)
	}
}

func TestEncodeRoundTrip(t *testing.T) {
	res, err := Crop(solid(30, 20, red), &Box{5, 5, 10, 10}, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(res.Image)
	if err != nil {
		t.Fatal(err)
	}
	got, err := png.Decode(bytes.NewReader(data))
	if err != nil || got.Bounds().Dx() != 10 || got.Bounds().Dy() != 10 {
		t.Fatalf("decoded %v, %v", got, err)
	}
}

func TestDiffSamePicturesHaveNoBoxes(t *testing.T) {
	a := solid(64, 64, white)
	res, err := Diff(a, solid(64, 64, white), def)
	if err != nil {
		t.Fatal(err)
	}
	if res.Differs || res.Percent != 0 || len(res.Boxes) != 0 || res.Boxes == nil {
		t.Fatalf("res = %+v", res)
	}
}

func TestDiffBoxesAndPercent(t *testing.T) {
	a := solid(200, 100, white)
	b := solid(200, 100, white)
	paint(b, Box{10, 10, 20, 10}, red)   // box one: 200 pixels, all different
	paint(b, Box{150, 60, 10, 10}, blue) // box two: 100 pixels
	res, err := Diff(a, b, def)
	if err != nil {
		t.Fatal(err)
	}
	want := []Region{
		{Box: Box{10, 10, 20, 10}, Pixels: 200, Percent: 100},
		{Box: Box{150, 60, 10, 10}, Pixels: 100, Percent: 100},
	}
	if !res.Differs || len(res.Boxes) != 2 || res.Boxes[0] != want[0] || res.Boxes[1] != want[1] {
		t.Fatalf("boxes = %+v, want %+v", res.Boxes, want)
	}
	if res.Percent != 1.5 { // 300 of 20000 pixels
		t.Fatalf("percent = %v, want 1.5", res.Percent)
	}
}

func TestDiffPartOfABoxGivesItsPercent(t *testing.T) {
	a := solid(100, 100, white)
	b := solid(100, 100, white)
	// Two dots 20 px apart sit in one box: 2 pixels of a 21x1 box.
	b.SetNRGBA(10, 40, red)
	b.SetNRGBA(30, 40, red)
	res, err := Diff(a, b, DiffOptions{Tile: 32})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Boxes) != 1 || res.Boxes[0].Box != (Box{10, 40, 21, 1}) || res.Boxes[0].Percent != 9.52 {
		t.Fatalf("boxes = %+v", res.Boxes)
	}
}

func TestDiffThresholdHidesNoise(t *testing.T) {
	a := solid(40, 40, color.NRGBA{100, 100, 100, 255})
	b := solid(40, 40, color.NRGBA{104, 100, 96, 255})
	res, err := Diff(a, b, def)
	if err != nil || res.Differs {
		t.Fatalf("noise below the threshold differs: %+v %v", res, err)
	}
	res, err = Diff(a, b, DiffOptions{Threshold: 2})
	if err != nil || !res.Differs || len(res.Boxes) != 1 || res.Boxes[0].Percent != 100 {
		t.Fatalf("noise above a low threshold: %+v %v", res, err)
	}
}

func TestDiffDifferentSizesCountsTheExtraArea(t *testing.T) {
	a := solid(100, 100, white)
	b := solid(100, 150, white)
	res, err := Diff(a, b, def)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Differs || len(res.Boxes) != 1 || res.Boxes[0].Box != (Box{0, 100, 100, 50}) || res.Boxes[0].Percent != 100 {
		t.Fatalf("res = %+v", res)
	}
	if res.WidthA != 100 || res.HeightB != 150 {
		t.Fatalf("sizes = %+v", res)
	}
}

func TestDiffRefusesBadOptions(t *testing.T) {
	a := solid(4, 4, white)
	if _, err := Diff(a, a, DiffOptions{Tile: -1}); err == nil {
		t.Error("negative tile accepted")
	}
	if _, err := Diff(a, a, DiffOptions{Threshold: 300}); err == nil {
		t.Error("threshold 300 accepted")
	}
}
