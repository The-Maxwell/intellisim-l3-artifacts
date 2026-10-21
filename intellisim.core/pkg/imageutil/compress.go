// Package imageutil provides small image helpers — currently base64 sprite compression for
// entity display images (decode → downscale to a max dimension → re-encode → base64).
package imageutil

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"

	// Register decoders for the supported input formats.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // webp decode only
)

// ErrTooLarge is returned when an image still exceeds the byte cap after compression.
var ErrTooLarge = errors.New("image too large after compression")

// ErrUnsupported is returned when the image bytes cannot be decoded.
var ErrUnsupported = errors.New("unsupported or undecodable image")

// Result is the compressed output: base64 bytes (no data: prefix), mime, and pixel size.
type Result struct {
	DataB64  string
	MimeType string
	Width    uint64
	Height   uint64
}

// Compress decodes a base64 image, downscales it so its largest side is at most maxDim, and
// re-encodes it (PNG to preserve transparency; JPEG when that is smaller / for opaque
// photos). It is idempotent: an image already within maxDim and the byte cap in a supported
// output format is returned unchanged. Returns ErrTooLarge if the result still exceeds
// maxBytes, or ErrUnsupported if the input cannot be decoded.
func Compress(dataB64 string, maxDim, maxBytes int) (Result, error) {
	raw, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	img, format, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Fast path: already small enough, within the byte cap, in a directly-storable format.
	if w <= maxDim && h <= maxDim && len(raw) <= maxBytes && (format == "png" || format == "jpeg") {
		return Result{
			DataB64:  dataB64,
			MimeType: "image/" + format,
			Width:    uint64(w),
			Height:   uint64(h),
		}, nil
	}

	dst := img
	nw, nh := w, h
	if w > maxDim || h > maxDim {
		nw, nh = scaledDims(w, h, maxDim)
		scaled := image.NewRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), img, b, xdraw.Over, nil)
		dst = scaled
	}

	opaque := isOpaque(dst)
	out, mime, err := encode(dst, opaque)
	if err != nil {
		return Result{}, err
	}
	// If a PNG of a (large) opaque image is still over the cap, fall back to JPEG.
	if len(out) > maxBytes && mime == "image/png" {
		if jout, jmime, jerr := encodeJPEG(dst); jerr == nil && len(jout) < len(out) {
			out, mime = jout, jmime
		}
	}
	if len(out) > maxBytes {
		return Result{}, fmt.Errorf("%w: %d bytes > %d", ErrTooLarge, len(out), maxBytes)
	}
	return Result{
		DataB64:  base64.StdEncoding.EncodeToString(out),
		MimeType: mime,
		Width:    uint64(nw),
		Height:   uint64(nh),
	}, nil
}

func encode(img image.Image, opaque bool) ([]byte, string, error) {
	if opaque {
		return encodeJPEG(img)
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

func encodeJPEG(img image.Image) ([]byte, string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 82}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

// scaledDims returns the largest dimensions that fit within maxDim preserving aspect ratio.
func scaledDims(w, h, maxDim int) (int, int) {
	if w <= 0 || h <= 0 {
		return w, h
	}
	if w >= h {
		nh := h * maxDim / w
		if nh < 1 {
			nh = 1
		}
		return maxDim, nh
	}
	nw := w * maxDim / h
	if nw < 1 {
		nw = 1
	}
	return nw, maxDim
}

// isOpaque reports whether every pixel is fully opaque (so JPEG is safe).
func isOpaque(img image.Image) bool {
	if oi, ok := img.(interface{ Opaque() bool }); ok {
		return oi.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}
