package fontstash

import (
	"testing"
)

func BenchmarkDrawText(b *testing.B) {
	mock := &MockRenderer{}
	fs, _ := New(Params{
		Width:    1024,
		Height:   1024,
		Renderer: mock,
	})
	fontNormal, _ := fs.AddFont("sans", "testdata/DroidSerif-Regular.ttf")
	fs.SetFont(fontNormal)
	fs.SetSize(24.0)
	fs.SetColor(0xffffffff)

	s := "The quick brown fox jumps over the lazy dog. 1234567890!@#$%^&*()"
	// Warm up to ensure glyphs are loaded
	fs.DrawText(0, 0, s)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fs.DrawText(10, 10, s)
	}
}

func BenchmarkTextBounds(b *testing.B) {
	mock := &MockRenderer{}
	fs, _ := New(Params{
		Width:    1024,
		Height:   1024,
		Renderer: mock,
	})
	fontNormal, _ := fs.AddFont("sans", "testdata/DroidSerif-Regular.ttf")
	fs.SetFont(fontNormal)
	fs.SetSize(24.0)

	s := "The quick brown fox jumps over the lazy dog. 1234567890!@#$%^&*()"
	// Warm up
	fs.TextBounds(0, 0, s, nil)

	var bounds [4]float32
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		fs.TextBounds(10, 10, s, &bounds)
	}
}
