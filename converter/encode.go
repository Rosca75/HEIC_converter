// converter/encode.go — format-specific encoders.
//
// One Go function per output format. Each takes an already-decoded
// image.Image and a 0–100 quality value, writes the encoded bytes to w, and
// returns an error. The dispatch table lives in converter.go.
package converter

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"github.com/gen2brain/webp"
	"golang.org/x/image/tiff"
)

// encodeJPEG writes img as JPEG with the given quality (1..100).
func encodeJPEG(w io.Writer, img image.Image, quality int) error {
	return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
}

// encodePNG writes img as PNG. PNG is lossless, so "quality" controls only
// the compression effort level.
func encodePNG(w io.Writer, img image.Image, quality int) error {
	var level png.CompressionLevel
	switch {
	case quality <= 33:
		level = png.NoCompression
	case quality <= 66:
		level = png.DefaultCompression
	default:
		level = png.BestCompression
	}
	enc := &png.Encoder{CompressionLevel: level}
	return enc.Encode(w, img)
}

// encodeTIFF writes img as TIFF. x/image/tiff supports only Uncompressed
// and Deflate; we toggle on the Deflate+Predictor combo for anything above
// the lowest third of the slider (Predictor gives a ~20% size reduction on
// photographic data for free).
func encodeTIFF(w io.Writer, img image.Image, quality int) error {
	opts := &tiff.Options{}
	if quality > 33 {
		opts.Compression = tiff.Deflate
		opts.Predictor = true
	}
	return tiff.Encode(w, img, opts)
}

// encodeWebP writes img as WebP. Quality is a true 0–100 lossy knob; Method
// 4 is the library's "balanced" speed/quality setting.
func encodeWebP(w io.Writer, img image.Image, quality int) error {
	return webp.Encode(w, img, webp.Options{
		Quality: quality,
		Method:  4,
	})
}

// encoderFor returns the encoder function and output file extension for the
// given format string. Returns an error for unknown formats.
func encoderFor(format string) (func(io.Writer, image.Image, int) error, string, error) {
	switch format {
	case "jpg", "jpeg":
		return encodeJPEG, "jpg", nil
	case "png":
		return encodePNG, "png", nil
	case "tiff", "tif":
		return encodeTIFF, "tiff", nil
	case "webp":
		return encodeWebP, "webp", nil
	}
	return nil, "", fmt.Errorf("unsupported output format: %s", format)
}
