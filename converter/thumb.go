package converter

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jdeng/goheif/heif"
	"github.com/rwcarlsen/goexif/exif"
)

// extractMetaFast reads HEIC container metadata and embedded thumbnail without
// decoding the full image. Returns zero width on failure (caller falls back).
func extractMetaFast(path string) (width, height int, camera, createdAt, thumbBase64 string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, "unknown", "", "", err
	}
	defer f.Close()

	hf := heif.Open(f)
	primary, err := hf.PrimaryItem()
	if err != nil {
		return 0, 0, "unknown", "", "", err
	}
	if w, h, ok := primary.SpatialExtents(); ok {
		width, height = w, h
	}

	camera = "unknown"
	exifBytes, exifErr := hf.EXIF()
	if exifErr == nil && len(exifBytes) > 0 {
		if x, decErr := exif.Decode(bytes.NewReader(exifBytes)); decErr == nil {
			if tag, e := x.Get(exif.Model); e == nil {
				if s, e2 := tag.StringVal(); e2 == nil {
					camera = strings.Trim(s, "\" \x00")
				}
			}
			if dt, e := x.DateTime(); e == nil {
				createdAt = dt.UTC().Format(time.RFC3339)
			}
		}
	}

	thumbBase64 = extractEmbeddedThumb(hf, primary)
	return width, height, camera, createdAt, thumbBase64, nil
}

// extractEmbeddedThumb scans HEIC item list for a "thmb" reference pointing to the
// primary item. Only returns data when bytes are a valid JPEG (FF D8 FF magic).
func extractEmbeddedThumb(hf *heif.File, primary *heif.Item) string {
	for id := uint32(1); id <= 50; id++ {
		item, err := hf.ItemByID(id)
		if err != nil || item.ID == primary.ID {
			continue
		}
		ref := item.Reference("thmb")
		if ref == nil {
			continue
		}
		for _, toID := range ref.ToItemIDs {
			if toID != primary.ID {
				continue
			}
			data, err := hf.GetItemData(item)
			if err != nil || len(data) < 3 {
				continue
			}
			// Verify JPEG SOI marker; non-JPEG thumbnails (e.g. HEVC) fall back to ImageMagick.
			if data[0] != 0xFF || data[1] != 0xD8 || data[2] != 0xFF {
				continue
			}
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
		}
	}
	return ""
}

// parseVerboseInfo extracts resolution and EXIF info using a single identify -verbose call.
func parseVerboseInfo(p string) (width, height int, camera, createdAt string) {
	out, err := exec.Command("magick", "identify", "-verbose", p+"[0]").CombinedOutput()
	if err != nil {
		return 0, 0, "unknown", ""
	}
	camera = "unknown"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Geometry:"); ok {
			fmt.Sscanf(strings.TrimSpace(after), "%dx%d", &width, &height)
		}
		if after, ok := strings.CutPrefix(line, "exif:Model:"); ok {
			camera = strings.TrimSpace(after)
		}
		if after, ok := strings.CutPrefix(line, "date:create:"); ok {
			createdAt = strings.TrimSpace(after)
		}
	}
	return width, height, camera, createdAt
}

// generateThumb returns a base64 data URL of a 48×48 JPEG thumbnail using ImageMagick.
func generateThumb(p string) string {
	cmd := exec.Command("magick", "convert", p+"[0]",
		"-thumbnail", "48x48^",
		"-gravity", "center",
		"-extent", "48x48",
		"jpg:-",
	)
	data, err := cmd.Output()
	if err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}
