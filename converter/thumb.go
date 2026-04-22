// converter/thumb.go — HEIC thumbnail extraction via the 192 KB fast-path.
//
// The Rosca75/heic decoder walks the ISOBMFF `iloc` box to locate the
// embedded thumbnail tile. If the tile's absolute file offset falls inside
// the buffer we hand it, no additional I/O is required — so reading only
// the first 192 KB of the file is enough for iPhone HEICs.
package converter

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"

	"github.com/Rosca75/heic"
	"golang.org/x/image/draw"
)

// heicHeaderReadSize is the byte-range size used by the HEIC fast path.
// 192 KB comfortably covers ftyp + meta + iloc + thumbnail tile on every
// iPhone HEIC tested; if a specific file needs more, we fall back to a
// full read in decodeHEICThumbnail.
const heicHeaderReadSize = 192 * 1024

// readHEICHeader reads up to heicHeaderReadSize bytes from path. Short files
// are fine — io.ReadFull returns ErrUnexpectedEOF and we just use however
// many bytes we actually got.
func readHEICHeader(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, heicHeaderReadSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	return buf[:n], nil
}

// decodeHEICThumbnail returns an image.Image thumbnail from a HEIC file.
// Tries, in order: header-only thumbnail decode → header-only primary decode
// (for files without an embedded thumbnail) → full-file thumbnail decode →
// full-file primary decode.
func decodeHEICThumbnail(path string) (image.Image, error) {
	header, err := readHEICHeader(path)
	if err != nil {
		return nil, err
	}
	if img, thumbErr := heic.DecodeThumbnail(bytes.NewReader(header)); thumbErr == nil {
		return img, nil
	}
	if img, decErr := heic.Decode(bytes.NewReader(header)); decErr == nil {
		return img, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if img, thumbErr := heic.DecodeThumbnail(bytes.NewReader(data)); thumbErr == nil {
		return img, nil
	}
	return heic.Decode(bytes.NewReader(data))
}

// heicThumbnailJPEG returns a JPEG-encoded, 48×48-max thumbnail for the
// gallery row. 48 px is enough for the table; we use ApproxBiLinear because
// it is ~10× faster than a manual img.At/Set loop and produces visibly
// better output at this size.
func heicThumbnailJPEG(path string) ([]byte, error) {
	img, err := decodeHEICThumbnail(path)
	if err != nil {
		return nil, err
	}
	out := resizeImageToJPEG(img, 48, 80)
	if out == nil {
		return nil, fmt.Errorf("jpeg encode failed")
	}
	return out, nil
}

// resizeImageToJPEG resizes img to fit inside maxDim×maxDim (preserving
// aspect ratio) and JPEG-encodes it at the given quality. Returns nil on
// encoder failure.
func resizeImageToJPEG(img image.Image, maxDim, quality int) []byte {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	newW, newH := srcW, srcH
	if srcW > maxDim || srcH > maxDim {
		if srcW >= srcH {
			newW = maxDim
			newH = srcH * maxDim / srcW
		} else {
			newH = maxDim
			newW = srcW * maxDim / srcH
		}
	}
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}
	dstRect := image.Rect(0, 0, newW, newH)
	thumb := image.NewRGBA(dstRect)
	draw.ApproxBiLinear.Scale(thumb, dstRect, img, b, draw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: quality}); err != nil {
		return nil
	}
	return buf.Bytes()
}
