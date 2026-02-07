package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
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

		minX := int(math.Floor(float64(v0.X)))
		minY := int(math.Floor(float64(v0.Y)))
		maxX := int(math.Ceil(float64(v1.X)))
		maxY := int(math.Ceil(float64(v1.Y)))

		texW := float32(r.Width)
		texH := float32(r.Height)

		sMinX := int(v0.U * texW)
		sMinY := int(v0.V * texH)

		c := v0.Color
		col := color.RGBA{
			R: uint8(c),       // R is LSB in C code usually for LE?
			G: uint8(c >> 8),
			B: uint8(c >> 16),
			A: uint8(c >> 24),
		}
		// Wait, in previous example I assumed Big Endian (RRGGBBAA)?
		// The C code uses:
		// unsigned int white = glfonsRGBA(255,255,255,255);
		// #define glfonsRGBA(r, g, b, a) (((unsigned int)r) | ((unsigned int)g << 8) | ((unsigned int)b << 16) | ((unsigned int)a << 24))
		// This creates ABGR (little endian uint32).
		// So R is lowest byte.
		// My previous example parsed as RRGGBBAA (Big Endian).
		// Let's use Little Endian ABGR as per C convention if we want 1:1.
		// R = c & 0xFF
		// G = (c >> 8) & 0xFF
		// B = (c >> 16) & 0xFF
		// A = (c >> 24) & 0xFF

		src := image.NewUniform(col)

		dr := image.Rect(minX, minY, maxX, maxY)
		sp := image.Point{sMinX, sMinY}

		draw.DrawMask(r.Result, dr, src, image.Point{}, r.Atlas, sp, draw.Over)
	}
}

func main() {
	width, height := 800, 600
	renderer := NewImageRenderer(512, 512)
	renderer.Result = image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill background
	// draw.Draw(renderer.Result, renderer.Result.Bounds(), image.White, image.Point{}, draw.Src)
	// Dark background
	bg := color.RGBA{50, 50, 50, 255}
	draw.Draw(renderer.Result, renderer.Result.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)

	fs, err := fontstash.New(fontstash.Params{
		Width:    512,
		Height:   512,
		Renderer: renderer,
		Flags:    fontstash.ZeroTopLeft, // Important for image/draw coordinate system!
	})
	if err != nil {
		log.Fatal(err)
	}

	fontNormal, err := fs.AddFont("sans", "fontstash/testdata/DroidSerif-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}

	// Helper color
	rgba := func(r, g, b, a uint8) uint32 {
		return uint32(r) | (uint32(g) << 8) | (uint32(b) << 16) | (uint32(a) << 24)
	}
	white := rgba(255, 255, 255, 255)
	brown := rgba(192, 128, 0, 255)
	blue := rgba(0, 192, 255, 255)
	black := rgba(0, 0, 0, 255)

	_ = black // unused

	dx, dy := float32(10.0), float32(100.0)

	fs.SetFont(fontNormal)
	fs.SetSize(124.0)
	fs.SetColor(white)
	dx = fs.DrawText(dx, dy, "The big ")

	fs.SetSize(24.0)
	fs.SetColor(brown)
	fs.DrawText(dx, dy, "brown fox")

	dx = 10.0 // Reset dx
	dy += 124.0
	fs.SetSize(20.0)
	fs.SetColor(white)
	fs.DrawText(dx, dy, "Jumps over the lazy dog.")

	dy += 50.0
	fs.SetSize(48.0)
	fs.SetColor(blue)
	fs.DrawText(dx, dy, "Blurry text")
	fs.SetBlur(5.0)
	fs.DrawText(dx+300, dy, "Blurry text")
	fs.SetBlur(0.0)

	dy += 80.0
	fs.SetSize(24.0)
	fs.SetColor(white)
	fs.SetAlign(fontstash.AlignLeft | fontstash.AlignTop)
	fs.DrawText(dx, dy, "Top Align")

	// Draw line
	lineColor := image.NewUniform(color.RGBA{255, 0, 0, 128})
	drawHorizontalLine(renderer.Result, int(dx), int(dy), 100, lineColor)

	fs.SetAlign(fontstash.AlignLeft | fontstash.AlignMiddle)
	fs.DrawText(dx+170, dy, "Middle Align")
	drawHorizontalLine(renderer.Result, int(dx)+170, int(dy), 100, lineColor)

	fs.SetAlign(fontstash.AlignLeft | fontstash.AlignBottom)
	fs.DrawText(dx+340, dy, "Bottom Align")
	drawHorizontalLine(renderer.Result, int(dx)+340, int(dy), 100, lineColor)

	fs.SetAlign(fontstash.AlignLeft | fontstash.AlignBaseline)
	fs.DrawText(dx+510, dy, "Baseline Align")
	drawHorizontalLine(renderer.Result, int(dx)+510, int(dy), 100, lineColor)

	// Save
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

func drawHorizontalLine(img *image.RGBA, x, y, w int, c *image.Uniform) {
	r := image.Rect(x, y, x+w, y+1)
	draw.Draw(img, r, c, image.Point{}, draw.Src)
}
