// Command appicon builds OmniThreads icons from one high-resolution render.
// PNG-backed ICNS entries cover standard and Retina sizes without external tools.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/draw"
)

func main() {
	source := flag.String("source", "assets/app-icon.svg", "vector source")
	output := flag.String("out", "assets/AppIcon.icns", "ICNS destination")
	preview := flag.String("preview", "", "optional 1024px PNG destination")
	flag.Parse()
	data, err := os.ReadFile(*source)
	if err != nil {
		log.Fatal(err)
	}
	// Render strokes once at native SVG resolution: the renderer does not scale
	// stroke widths consistently when SetTarget renders directly at small sizes.
	svg, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.StrictErrorMode)
	if err != nil {
		log.Fatal(err)
	}
	master := image.NewRGBA(image.Rect(0, 0, 1024, 1024))
	svg.SetTarget(0, 0, 1024, 1024)
	svg.Draw(rasterx.NewDasher(1024, 1024, rasterx.NewScannerGV(1024, 1024, master, master.Bounds())), 1)
	var payload bytes.Buffer
	for _, entry := range []struct {
		tag  string
		size int
	}{{"icp4", 16}, {"icp5", 32}, {"icp6", 64}, {"ic07", 128}, {"ic08", 256}, {"ic09", 512}, {"ic10", 1024}, {"ic11", 32}, {"ic12", 64}, {"ic13", 256}, {"ic14", 512}} {
		img := image.NewRGBA(image.Rect(0, 0, entry.size, entry.size))
		draw.CatmullRom.Scale(img, img.Bounds(), master, master.Bounds(), draw.Src, nil)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			log.Fatal(err)
		}
		payload.WriteString(entry.tag)
		_ = binary.Write(&payload, binary.BigEndian, uint32(encoded.Len()+8))
		payload.Write(encoded.Bytes())
		if entry.size == 1024 && *preview != "" {
			write(*preview, encoded.Bytes())
		}
	}
	var result bytes.Buffer
	result.WriteString("icns")
	_ = binary.Write(&result, binary.BigEndian, uint32(payload.Len()+8))
	result.Write(payload.Bytes())
	write(*output, result.Bytes())
}
func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Fatal(err)
	}
}
