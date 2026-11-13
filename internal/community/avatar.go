package community

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

func validateAvatar(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 64*1024 {
		return errors.New("头像过大，请重新选择图片")
	}
	encoded, ok := strings.CutPrefix(value, "data:image/jpeg;base64,")
	expected := "jpeg"
	if !ok {
		encoded, ok = strings.CutPrefix(value, "data:image/png;base64,")
		expected = "png"
	}
	if !ok {
		return errors.New("头像格式无效")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return errors.New("头像格式无效")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != expected || config.Width < 1 || config.Height < 1 || config.Width > 256 || config.Height > 256 {
		return errors.New("头像格式或尺寸无效")
	}
	if _, _, err = image.Decode(bytes.NewReader(data)); err != nil {
		return errors.New("头像图片已损坏")
	}
	return nil
}
