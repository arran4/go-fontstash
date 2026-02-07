package fontstash

import (
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// AddFont loads a font from a file.
func (fs *FontStash) AddFont(name, path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}
	return fs.AddFontFromBytes(name, data)
}

// AddFontFromBytes loads a font from memory.
func (fs *FontStash) AddFontFromBytes(name string, data []byte) (int, error) {
	f, err := opentype.Parse(data)
	if err != nil {
		return -1, err
	}

	// Create a temporary face to get metrics
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    1000, // Large enough to minimize rounding errors
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		return -1, err
	}
	defer face.Close()

	metrics := face.Metrics()

	// We use fixed point arithmetic for precision, but convert to float for storage
	// Ascent is positive (up), Descent is positive (down) in Go font.Metrics.
	// We convert Descent to negative coordinate to match C fontstash behavior.
	ascent := float32(metrics.Ascent)
	descent := -float32(metrics.Descent)
	height := float32(metrics.Height)

	fh := ascent - descent
	if fh == 0 {
		fh = 1
	}

	fontObj := &Font{
		Name:       name,
		Data:       data,
		sfnt:       f,
		Ascender:   ascent / fh,
		Descender:  descent / fh,
		LineHeight: height / fh,
		Glyphs:     make([]Glyph, 0, 256),
		Lut:        make([]int, 256),
		Fallbacks:  make([]int, 0),
	}

	// Init hash lookup
	for i := range fontObj.Lut {
		fontObj.Lut[i] = -1
	}

	fs.Fonts = append(fs.Fonts, fontObj)
	return len(fs.Fonts) - 1, nil
}

// AddFallbackFont adds a fallback font to a base font.
func (fs *FontStash) AddFallbackFont(base, fallback int) bool {
	if base < 0 || base >= len(fs.Fonts) || fallback < 0 || fallback >= len(fs.Fonts) {
		return false
	}
	fs.Fonts[base].Fallbacks = append(fs.Fonts[base].Fallbacks, fallback)
	return true
}

// getGlyphIndex returns the glyph index for a codepoint.
func (fs *FontStash) getGlyphIndex(f *Font, codepoint rune) int {
	index, err := f.sfnt.GlyphIndex(nil, codepoint)
	if err != nil {
		return 0
	}
	return int(index)
}

func (fs *FontStash) getGlyphKernAdvance(f *Font, glyph1, glyph2 int, size float32) int {
	ppem := fixed.Int26_6(size * 64)
	k, err := f.sfnt.Kern(nil, sfnt.GlyphIndex(glyph1), sfnt.GlyphIndex(glyph2), ppem, font.HintingFull)
	if err != nil {
		return 0
	}

	// Round to integer pixels?
	// C code: return (int)((ftKerning.x + 32) >> 6); -> Round
	return k.Round()
}
