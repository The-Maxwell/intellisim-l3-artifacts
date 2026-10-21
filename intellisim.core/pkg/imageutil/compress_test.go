package imageutil

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngB64(w, h int, withAlpha bool) string {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if withAlpha && (x+y)%2 == 0 {
				a = 0
			}
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 100, A: a})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestCompress_Downscale(t *testing.T) {
	in := pngB64(1024, 512, false)
	res, err := Compress(in, 256, 1<<20)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.Width != 256 || res.Height != 128 {
		t.Errorf("downscaled dims: got %dx%d want 256x128", res.Width, res.Height)
	}
	// opaque image should re-encode to JPEG
	if res.MimeType != "image/jpeg" {
		t.Errorf("opaque should be JPEG, got %s", res.MimeType)
	}
	decoded, _ := base64.StdEncoding.DecodeString(res.DataB64)
	if len(decoded) == 0 {
		t.Errorf("empty output")
	}
}

func TestCompress_PreservesAlphaAsPNG(t *testing.T) {
	in := pngB64(512, 512, true)
	res, err := Compress(in, 256, 1<<20)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.MimeType != "image/png" {
		t.Errorf("alpha image should stay PNG, got %s", res.MimeType)
	}
	if res.Width != 256 || res.Height != 256 {
		t.Errorf("dims: got %dx%d", res.Width, res.Height)
	}
}

func TestCompress_FastPathSmallPNG(t *testing.T) {
	in := pngB64(64, 64, true)
	res, err := Compress(in, 256, 1<<20)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.DataB64 != in {
		t.Errorf("small in-spec PNG should pass through unchanged")
	}
	if res.MimeType != "image/png" || res.Width != 64 || res.Height != 64 {
		t.Errorf("fast path metadata wrong: %s %dx%d", res.MimeType, res.Width, res.Height)
	}
}

func TestCompress_TooLarge(t *testing.T) {
	in := pngB64(512, 512, true) // alpha → PNG, won't shrink under a tiny cap
	_, err := Compress(in, 256, 64)
	if err == nil {
		t.Fatalf("expected ErrTooLarge")
	}
}

func TestCompress_Unsupported(t *testing.T) {
	if _, err := Compress(base64.StdEncoding.EncodeToString([]byte("not an image")), 256, 1<<20); err == nil {
		t.Fatalf("expected ErrUnsupported")
	}
}
