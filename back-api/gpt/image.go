package gpt

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"strings"

	xdraw "golang.org/x/image/draw"
)

// maxImageDimension is the target long-side limit (px) for images sent to the
// vision model. ECG grids stay legible well below this, while the smaller
// payload reduces base64 size and vision-call latency. Chosen within the
// recommended 1800-2200px band.
const maxImageDimension = 2000

// jpegQuality balances readability of the ECG grid against payload size.
const jpegQuality = 85

// downscaleImage re-encodes data as a JPEG whose longest side is at most
// maxImageDimension. It returns (downscaled, true, nil) when the image was
// resized, or (data, false, nil) when no change was needed (already small
// enough or an unsupported/undecodable format). Decode/encode failures return
// the original bytes with a non-nil error so callers can fall back safely.
func downscaleImage(data []byte) (out []byte, changed bool, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		// Unknown or unsupported format (e.g. GIF/WebP not registered); leave
		// the original untouched.
		return data, false, nil
	}
	if cfg.Width <= maxImageDimension && cfg.Height <= maxImageDimension {
		return data, false, nil
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, false, fmt.Errorf("decode image for downscale: %w", err)
	}

	dstW, dstH := scaledDimensions(cfg.Width, cfg.Height, maxImageDimension)
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return data, false, fmt.Errorf("encode downscaled image: %w", err)
	}
	return buf.Bytes(), true, nil
}

// scaledDimensions returns width/height scaled so the longest side equals maxDim,
// preserving aspect ratio. The shorter side is rounded to at least 1px.
func scaledDimensions(w, h, maxDim int) (int, int) {
	if w >= h {
		nh := max(int(float64(h)*float64(maxDim)/float64(w)), 1)
		return maxDim, nh
	}
	nw := max(int(float64(w)*float64(maxDim)/float64(h)), 1)
	return nw, maxDim
}

// isResizableImageType reports whether the content type is one we can decode and
// re-encode for downscaling.
func isResizableImageType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "image/jpeg") ||
		strings.HasPrefix(ct, "image/jpg") ||
		strings.HasPrefix(ct, "image/png")
}
