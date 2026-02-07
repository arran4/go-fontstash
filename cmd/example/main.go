package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/yourname/fontstash/fontstash"
)

type ImageRenderer struct {
	Width, Height int
	Atlas         *image.Alpha
	Result        *image.RGBA
}

func NewImageRenderer(width, height int) *ImageRenderer {
	return &ImageRenderer{
		Width:  width,
		Height: height,
		Atlas:  image.NewAlpha(image.Rect(0, 0, width, height)),
		Result: image.NewRGBA(image.Rect(0, 0, width, height)),
	}
}

func (r *ImageRenderer) Resize(width, height int) {
	r.Width = width
	r.Height = height
	// Create new atlas. We don't need to copy old data because fontstash will Update the full used area after resize.
	r.Atlas = image.NewAlpha(image.Rect(0, 0, width, height))
}

func (r *ImageRenderer) Update(rect image.Rectangle, data []byte, imgWidth int) {
	// Update atlas texture
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			idx := y*imgWidth + x
			if idx < len(data) {
				r.Atlas.SetAlpha(x, y, color.Alpha{A: data[idx]})
			}
		}
	}
}

func (r *ImageRenderer) Draw(verts []fontstash.Vertex) {
	for i := 0; i < len(verts); i += 6 {
		v0 := verts[i]   // TL
		v1 := verts[i+1] // BR

		// Destination rect
		minX := int(v0.X)
		minY := int(v0.Y)
		maxX := int(v1.X)
		maxY := int(v1.Y)

		// Source rect (texels)
		texW := float32(r.Width)
		texH := float32(r.Height)

		sMinX := int(v0.U * texW)
		sMinY := int(v0.V * texH)

		// Color (RGBA)
		c := v0.Color
		// Assume 0xRRGGBBAA for this example
		col := color.RGBA{
			R: uint8(c >> 24),
			G: uint8(c >> 16),
			B: uint8(c >> 8),
			A: uint8(c),
		}

		src := image.NewUniform(col)

		// DrawMask
		dr := image.Rect(minX, minY, maxX, maxY)
		sp := image.Point{sMinX, sMinY}

		draw.DrawMask(r.Result, dr, src, image.Point{}, r.Atlas, sp, draw.Over)
	}
}

func main() {
	width, height := 800, 600
	renderer := NewImageRenderer(512, 512) // Atlas size
	renderer.Result = image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill background with black
	draw.Draw(renderer.Result, renderer.Result.Bounds(), image.Black, image.Point{}, draw.Src)

	fs, err := fontstash.New(fontstash.Params{
		Width:    512,
		Height:   512,
		Renderer: renderer,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Load font
	fontNormal, err := fs.AddFont("sans", "fontstash/testdata/DroidSerif-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}

	fs.SetFont(fontNormal)
	fs.SetSize(48.0)
	fs.SetColor(0xFFFFFFFF) // White
	fs.DrawText(50, 100, "Hello FontStash in Go!")

	fs.SetSize(24.0)
	fs.SetColor(0xFF0000FF) // Red
	fs.DrawText(50, 160, "This is a pure Go port.")

    fs.SetSize(32.0)
    fs.SetColor(0x00FF00FF) // Green
    fs.DrawText(50, 220, "With atlas packing and software rendering.")

	// Save image
	f, err := os.Create("screenshot.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	if err := png.Encode(f, renderer.Result); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Saved screenshot.png")
}
