package gpt

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestDownscaleImage_LargeImageResized(t *testing.T) {
	data := encodePNG(t, 4000, 3000)

	out, changed, err := downscaleImage(data)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected large image to be downscaled")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode downscaled: %v", err)
	}
	if cfg.Width != maxImageDimension {
		t.Errorf("expected long side %d, got %dx%d", maxImageDimension, cfg.Width, cfg.Height)
	}
	if cfg.Height >= 3000 {
		t.Errorf("height should shrink proportionally, got %d", cfg.Height)
	}
	// Output must be valid JPEG.
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("downscaled output is not valid JPEG: %v", err)
	}
}

func TestDownscaleImage_SmallImageUntouched(t *testing.T) {
	data := encodePNG(t, 800, 600)

	out, changed, err := downscaleImage(data)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("small image should not be resized")
	}
	if !bytes.Equal(out, data) {
		t.Error("small image bytes should be returned unchanged")
	}
}

func TestDownscaleImage_PortraitAspectPreserved(t *testing.T) {
	data := encodePNG(t, 1500, 3000)

	out, changed, err := downscaleImage(data)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected resize")
	}
	cfg, _, _ := image.DecodeConfig(bytes.NewReader(out))
	if cfg.Height != maxImageDimension {
		t.Errorf("portrait long side should be %d, got height %d", maxImageDimension, cfg.Height)
	}
	if cfg.Width != 1000 {
		t.Errorf("expected proportional width 1000, got %d", cfg.Width)
	}
}

func TestDownscaleImage_GarbageReturnedUnchanged(t *testing.T) {
	data := []byte("not an image")
	out, changed, err := downscaleImage(data)
	if err != nil {
		t.Fatalf("undecodable input should not error, got %v", err)
	}
	if changed || !bytes.Equal(out, data) {
		t.Error("undecodable input should be returned unchanged")
	}
}
