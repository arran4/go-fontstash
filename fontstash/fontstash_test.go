// Copyright (c) 2013 Mikko Mononen memon@inside.org
//
// This software is provided 'as-is', without any express or implied
// warranty.  In no event will the authors be held liable for any damages
// arising from the use of this software.
//
// Permission is granted to anyone to use this software for any purpose,
// including commercial applications, and to alter it and redistribute it
// freely, subject to the following restrictions:
//
// 1. The origin of this software must not be misrepresented; you must not
//    claim that you wrote the original software. If you use this software
//    in a product, an acknowledgment in the product documentation would be
//    appreciated but is not required.
// 2. Altered source versions must be plainly marked as such, and must not be
//    misrepresented as being the original software.
// 3. This notice may not be removed or altered from any source distribution.
//
// This is an automated port of the original C library.

package fontstash

import (
	"image"
	"testing"
)

type MockRenderer struct {
	Updates int
	Draws   int
	Verts   int
}

func (r *MockRenderer) Resize(width, height int) {}
func (r *MockRenderer) Update(rect image.Rectangle, data []byte, imgWidth int) {
	r.Updates++
}
func (r *MockRenderer) Draw(verts []Vertex) {
	r.Draws++
	r.Verts += len(verts)
}

func TestDrawText(t *testing.T) {
	mock := &MockRenderer{}
	fs, err := New(Params{
		Width:    512,
		Height:   512,
		Renderer: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create fontstash: %v", err)
	}

	fontNormal, err := fs.AddFont("sans", "testdata/DroidSerif-Regular.ttf")
	if err != nil {
		t.Fatalf("Failed to load font: %v", err)
	}

	fs.SetFont(fontNormal)
	fs.SetSize(24.0)
	fs.SetColor(0xffffffff)

	fs.DrawText(10, 10, "Hello World")

	if mock.Draws == 0 {
		t.Errorf("Expected draws, got 0")
	}
	if mock.Verts == 0 {
		t.Errorf("Expected verts, got 0")
	}

    t.Logf("Draws: %d, Verts: %d", mock.Draws, mock.Verts)
}

func TestTextBounds(t *testing.T) {
	mock := &MockRenderer{}
	fs, err := New(Params{
		Width:    512,
		Height:   512,
		Renderer: mock,
	})
	if err != nil {
		t.Fatalf("Failed to create fontstash: %v", err)
	}

	fontNormal, err := fs.AddFont("sans", "testdata/DroidSerif-Regular.ttf")
	if err != nil {
		t.Fatalf("Failed to load font: %v", err)
	}

	fs.SetFont(fontNormal)
	fs.SetSize(24.0)

	var bounds [4]float32
	width := fs.TextBounds(10, 10, "Hello", &bounds)

	if width <= 0 {
		t.Errorf("Expected width > 0, got %f", width)
	}

	if bounds[2] <= bounds[0] {
		t.Errorf("Expected maxx > minx")
	}

    t.Logf("Width: %f, Bounds: %v", width, bounds)
}

func TestFlushMemoryLeak(t *testing.T) {
	mock := &MockRenderer{}
	fs, _ := New(Params{Width: 512, Height: 512, Renderer: mock})
	fontNormal, err := fs.AddFont("sans", "testdata/DroidSerif-Regular.ttf")
	if err != nil {
		t.Fatalf("Failed to load font: %v", err)
	}
	fs.SetFont(fontNormal)

	fs.DrawText(0, 0, "A")

    if len(fs.Verts) != 0 {
        t.Errorf("Expected Verts to be empty after flush, got %d", len(fs.Verts))
    }
	if cap(fs.Verts) == 0 {
		// Just noting that capacity might be non-zero, which is expected.
	}
}
