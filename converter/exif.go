// converter/exif.go — unified EXIF extraction via bep/imagemeta.
//
// bep/imagemeta handles JPEG, TIFF, WebP, PNG, HEIF/HEIC, AVIF, DNG, CR2,
// NEF, ARW — a superset of rwcarlsen/goexif's JPEG/TIFF-only coverage, and
// runs ~2.6× faster with ~40× fewer allocations. It is the only EXIF reader
// we use in this project.
package converter

import (
	"io"
	"strings"
	"time"

	"github.com/bep/imagemeta"
)

// extractExifInto populates the EXIF fields on meta from the HEIF stream in r.
// r must implement io.ReadSeeker (os.File and bytes.NewReader both do). Fields
// already set on meta are not overwritten — this lets callers pre-seed values
// from the ISOBMFF container before invoking this helper.
func extractExifInto(r io.ReadSeeker, meta *FileMeta) {
	var make_, model string

	_, _ = imagemeta.Decode(imagemeta.Options{
		R:           r,
		ImageFormat: imagemeta.HEIF,
		HandleTag: func(ti imagemeta.TagInfo) error {
			switch ti.Tag {
			case "DateTimeOriginal":
				if meta.CreatedAt == "" {
					switch v := ti.Value.(type) {
					case string:
						if t, err := time.Parse("2006:01:02 15:04:05", v); err == nil {
							meta.CreatedAt = t.UTC().Format(time.RFC3339)
						}
					case time.Time:
						if !v.IsZero() {
							meta.CreatedAt = v.UTC().Format(time.RFC3339)
						}
					}
				}
			case "Make":
				if s, ok := ti.Value.(string); ok {
					make_ = strings.TrimSpace(s)
				}
			case "Model":
				if s, ok := ti.Value.(string); ok {
					model = strings.TrimSpace(s)
				}
			}
			return nil
		},
	})

	if meta.Camera == "" || meta.Camera == "unknown" {
		combined := strings.TrimSpace(make_ + " " + model)
		if combined != "" {
			meta.Camera = combined
		}
	}
}
