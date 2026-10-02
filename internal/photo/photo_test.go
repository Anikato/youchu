package photo

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/rwcarlsen/goexif/exif"
)

func TestDetectHEIC(t *testing.T) {
	ftyp := []byte{
		0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p',
		'h', 'e', 'i', 'c', 0x00, 0x00, 0x00, 0x00,
		'm', 'i', 'f', '1', 'h', 'e', 'i', 'c',
	}
	if err := Detect(ftyp); !errors.Is(err, ErrHEIC) {
		t.Fatalf("err=%v", err)
	}
}

func TestTranscodeJPEGPNGWebP(t *testing.T) {
	jpegBytes := encodeJPEG(t, 20, 10)
	pngBytes := encodePNG(t, 8, 8)
	webpBytes := oneByOneWebP()
	for i, src := range [][]byte{jpegBytes, pngBytes, webpBytes} {
		out, err := Transcode(src)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		if out.Width < 1 || out.Height < 1 || len(out.Original) == 0 || len(out.Thumb) == 0 {
			t.Fatalf("%d empty %#v", i, out)
		}
		if _, err := jpeg.Decode(bytes.NewReader(out.Original)); err != nil {
			t.Fatalf("original jpeg %d: %v", i, err)
		}
		if _, err := jpeg.Decode(bytes.NewReader(out.Thumb)); err != nil {
			t.Fatalf("thumb jpeg %d: %v", i, err)
		}
		if hasOrientation(out.Original) || hasOrientation(out.Thumb) {
			t.Fatalf("%d stored orientation still set", i)
		}
	}
}

func TestTranscodeRejectsRandomAndHugeHeader(t *testing.T) {
	if _, err := Transcode([]byte("not-an-image")); !errors.Is(err, ErrNotImage) {
		t.Fatalf("random err=%v", err)
	}
	if _, err := Transcode(jpegSOF(10001, 5001)); !errors.Is(err, ErrTooManyPixels) {
		t.Fatalf("pixels err=%v", err)
	}
}

func TestTranscodeDownscaleAndOrientation6(t *testing.T) {
	big := encodeJPEG(t, 5000, 1000)
	out, err := Transcode(big)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 4096 {
		t.Fatalf("width=%d", out.Width)
	}
	src := jpegWithOrientation6(t, 30, 40)
	out, err = Transcode(src)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 40 || out.Height != 30 {
		t.Fatalf("oriented size %dx%d", out.Width, out.Height)
	}
	if hasOrientation(out.Original) {
		t.Fatal("orientation left in file")
	}
}

func TestPlaceWritesUniqueTmpThenFinals(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDirs(dir); err != nil {
		t.Fatal(err)
	}
	original := []byte("original-bytes")
	thumb := []byte("thumb-bytes")
	if err := Place(dir, 42, original, thumb); err != nil {
		t.Fatal(err)
	}
	gotOrig, err := os.ReadFile(filepath.Join(dir, "originals", "42.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	gotThumb, err := os.ReadFile(filepath.Join(dir, "thumbnails", "42.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotOrig, original) || !bytes.Equal(gotThumb, thumb) {
		t.Fatalf("stored bytes orig=%q thumb=%q", gotOrig, gotThumb)
	}
	ents, err := os.ReadDir(filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("tmp leftover %v", ents)
	}
}

func TestPlaceFailureCleansTmpAndRenamed(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDirs(dir); err != nil {
		t.Fatal(err)
	}
	block := filepath.Join(dir, "thumbnails", "3.jpg")
	if err := os.Mkdir(block, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Place(dir, 3, []byte("original-bytes"), []byte("thumb-bytes")); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(dir, "originals", "3.jpg")); !os.IsNotExist(err) {
		t.Fatal("original leftover")
	}
	ents, err := os.ReadDir(filepath.Join(dir, "tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("tmp leftover %v", ents)
	}
}

func TestRemoveDeletesBothAndIgnoresMissing(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureDirs(dir); err != nil {
		t.Fatal(err)
	}
	if err := Place(dir, 9, []byte("original-bytes"), []byte("thumb-bytes")); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir, 9); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "originals", "9.jpg")); !os.IsNotExist(err) {
		t.Fatal("original still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "thumbnails", "9.jpg")); !os.IsNotExist(err) {
		t.Fatal("thumb still there")
	}
	if err := Remove(dir, 9); err != nil {
		t.Fatal(err)
	}
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, m, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, m); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func oneByOneWebP() []byte {
	return []byte{
		0x52, 0x49, 0x46, 0x46, 0x1c, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
		0x56, 0x50, 0x38, 0x4c, 0x0f, 0x00, 0x00, 0x00, 0x2f, 0x00, 0x00, 0x00,
		0x00, 0x07, 0x10, 0xfd, 0x8f, 0xfe, 0x07, 0x22, 0xa2, 0xff, 0x01, 0x00,
	}
}

func jpegSOF(w, h int) []byte {
	// JFIF APP0 is required: image/jpeg DecodeConfig only returns at SOF for JFIF, otherwise it waits for SOS.
	return []byte{
		0xFF, 0xD8,
		0xFF, 0xE0, 0x00, 0x10,
		'J', 'F', 'I', 'F', 0x00,
		0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00,
		0xFF, 0xC0, 0x00, 0x0B, 0x08,
		byte(h >> 8), byte(h),
		byte(w >> 8), byte(w),
		0x01, 0x01, 0x11, 0x00,
		0xFF, 0xD9,
	}
}

func jpegWithOrientation6(t *testing.T, w, h int) []byte {
	t.Helper()
	raw := encodeJPEG(t, w, h)
	if len(raw) < 2 || raw[0] != 0xFF || raw[1] != 0xD8 {
		t.Fatal("not jpeg")
	}
	app1 := []byte{
		0xFF, 0xE1, 0x00, 0x22,
		'E', 'x', 'i', 'f', 0x00, 0x00,
		'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	out := append([]byte{raw[0], raw[1]}, app1...)
	out = append(out, raw[2:]...)
	if !hasOrientation(out) {
		t.Fatal("fixture missing orientation 6")
	}
	return out
}

func hasOrientation(b []byte) bool {
	x, err := exif.Decode(bytes.NewReader(b))
	if err != nil {
		return false
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return false
	}
	v, err := tag.Int(0)
	if err != nil {
		return false
	}
	return v >= 2 && v <= 8
}
