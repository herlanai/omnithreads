package avatarpicker

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"

	"github.com/herlanai/omnigo"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type Request struct {
	Language string `json:"language"`
}
type Result struct {
	Avatar   string `json:"avatar,omitempty"`
	Canceled bool   `json:"canceled"`
}
type OpenDialog func(context.Context, omnigo.FileDialogOptions) (omnigo.FileDialogResult, error)

// No path is accepted from RPC: only the native dialog can select the input file.
func Pick(ctx context.Context, req Request, open OpenDialog) (Result, error) {
	title := "选择 Agent 头像"
	button := "选择图片"
	if req.Language == "en" {
		title = "Choose Agent avatar"
		button = "Choose image"
	}
	result, err := open(ctx, omnigo.FileDialogOptions{Title: title, ButtonLabel: button, Filters: []omnigo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}})
	if err != nil {
		return Result{}, errors.New("无法打开图片选择窗口，请重试")
	}
	if result.Canceled || len(result.Paths) == 0 {
		return Result{Canceled: true}, nil
	}
	f, err := os.Open(result.Paths[0])
	if err != nil {
		return Result{}, errors.New("无法读取这张图片，请选择其他图片")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, errors.New("请选择 PNG、JPG 或 WebP 图片")
	}
	b, err := io.ReadAll(io.LimitReader(f, 10*1024*1024+1))
	if err != nil {
		return Result{}, errors.New("无法读取这张图片，请选择其他图片")
	}
	if len(b) > 10*1024*1024 {
		return Result{}, errors.New("请选择不超过 10 MB 的图片")
	}
	avatar, err := Prepare(b)
	return Result{Avatar: avatar}, err
}
func Prepare(b []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png" && format != "webp") {
		return "", errors.New("请选择 PNG、JPG 或 WebP 图片")
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > 40_000_000 {
		return "", errors.New("图片尺寸过大，请选择较小图片")
	}
	source, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return "", errors.New("无法读取这张图片，请选择其他图片")
	}
	bounds := source.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	x := bounds.Min.X + (bounds.Dx()-side)/2
	y := bounds.Min.Y + (bounds.Dy()-side)/2
	dest := image.NewRGBA(image.Rect(0, 0, 192, 192))
	draw.Draw(dest, dest.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dest, dest.Bounds(), source, image.Rect(x, y, x+side, y+side), draw.Over, nil)
	var out bytes.Buffer
	if err = jpeg.Encode(&out, dest, &jpeg.Options{Quality: 85}); err != nil {
		return "", errors.New("无法处理头像图片")
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(out.Bytes()), nil
}
