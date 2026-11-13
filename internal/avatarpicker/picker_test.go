package avatarpicker

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/herlanai/omnigo"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeSelectionAndCancel(t *testing.T) {
	var imageBytes bytes.Buffer
	png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 400, 200)))
	path := filepath.Join(t.TempDir(), "avatar.png")
	os.WriteFile(path, imageBytes.Bytes(), 0600)
	result, err := Pick(context.Background(), Request{Language: "en"}, func(_ context.Context, o omnigo.FileDialogOptions) (omnigo.FileDialogResult, error) {
		if o.Multiple || len(o.Filters) != 1 || o.Title != "Choose Agent avatar" {
			t.Fatal("incorrect native picker options")
		}
		return omnigo.FileDialogResult{Paths: []string{path}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(result.Avatar, "data:image/jpeg;base64,"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || kind != "jpeg" || cfg.Width != 192 || cfg.Height != 192 {
		t.Fatal("invalid processed avatar")
	}
	result, err = Pick(context.Background(), Request{}, func(context.Context, omnigo.FileDialogOptions) (omnigo.FileDialogResult, error) {
		return omnigo.FileDialogResult{Canceled: true}, nil
	})
	if err != nil || !result.Canceled || result.Avatar != "" {
		t.Fatal("cancel was not respected")
	}
	if _, err = Prepare([]byte("not an image")); err == nil {
		t.Fatal("invalid file accepted")
	}
}
