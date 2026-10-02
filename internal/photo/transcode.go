package photo

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png"

	"github.com/rwcarlsen/goexif/exif"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	MaxUploadBytes = 40 * 1024 * 1024 // 41943040
	MaxPixels      = 50_000_000
	MaxLongEdge    = 4096
	ThumbLongEdge  = 400
	JPEGQuality    = 85
)

type Result struct {
	Width    int
	Height   int
	Original []byte
	Thumb    []byte
}

func Transcode(src []byte) (Result, error) {
	if err := Detect(src); err != nil {
		return Result{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(src))
	if err != nil {
		return Result{}, ErrNotImage
	}
	if tooManyPixels(cfg.Width, cfg.Height) {
		return Result{}, ErrTooManyPixels
	}
	img, _, err := image.Decode(bytes.NewReader(src))
	if err != nil {
		return Result{}, ErrNotImage
	}
	b := img.Bounds()
	if tooManyPixels(b.Dx(), b.Dy()) {
		return Result{}, ErrTooManyPixels
	}
	if isJPEG(src) {
		img = applyJPEGOrientation(src, img)
	}
	oriented := flattenWhite(img)
	stored := fitLongEdge(oriented, MaxLongEdge)
	original, err := encodeJPEGBytes(stored)
	if err != nil {
		return Result{}, err
	}
	thumb, err := encodeJPEGBytes(fitLongEdge(stored, ThumbLongEdge))
	if err != nil {
		return Result{}, err
	}
	sb := stored.Bounds()
	return Result{
		Width:    sb.Dx(),
		Height:   sb.Dy(),
		Original: original,
		Thumb:    thumb,
	}, nil
}

func tooManyPixels(w, h int) bool {
	if w < 1 || h < 1 {
		return true
	}
	return int64(w)*int64(h) > MaxPixels
}

func applyJPEGOrientation(src []byte, img image.Image) image.Image {
	x, err := exif.Decode(bytes.NewReader(src))
	if err != nil {
		return img
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return img
	}
	o, err := tag.Int(0)
	if err != nil || o < 2 || o > 8 {
		return img
	}
	return orient(img, o)
}

func orient(src image.Image, o int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	var dst *image.NRGBA
	switch o {
	case 2, 3, 4:
		dst = image.NewNRGBA(image.Rect(0, 0, w, h))
	default:
		dst = image.NewNRGBA(image.Rect(0, 0, h, w))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch o {
			case 2:
				dst.Set(w-1-x, y, c)
			case 3:
				dst.Set(w-1-x, h-1-y, c)
			case 4:
				dst.Set(x, h-1-y, c)
			case 5:
				dst.Set(y, x, c)
			case 6:
				dst.Set(h-1-y, x, c)
			case 7:
				dst.Set(h-1-y, w-1-x, c)
			case 8:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

func flattenWhite(src image.Image) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Over)
	return dst
}

func fitLongEdge(src *image.NRGBA, maxEdge int) *image.NRGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 1 || h < 1 {
		return src
	}
	long := w
	if h > long {
		long = h
	}
	if long <= maxEdge {
		return src
	}
	nw := int(int64(w) * int64(maxEdge) / int64(long))
	nh := int(int64(h) * int64(maxEdge) / int64(long))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Src, nil)
	return dst
}

func encodeJPEGBytes(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
