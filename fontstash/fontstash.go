package fontstash

import (
	"image"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Renderer handles backend-specific operations.
type Renderer interface {
	Resize(width, height int)
	Update(rect image.Rectangle, data []byte, imgWidth int)
	Draw(verts []Vertex)
}

// Vertex represents a vertex in the quad.
type Vertex struct {
	X, Y, U, V float32
	Color      uint32
}

// Glyph represents a glyph in the atlas.
type Glyph struct {
	Codepoint  rune
	Index      int // Glyph index in the font
	Size       int16
	Blur       int16
	X0, Y0     int16
	X1, Y1     int16
	XAdv       int16
	XOff, YOff int16
	Next       int // Index of next glyph in hash chain
}

// Font represents a loaded font.
type Font struct {
	Name       string
	Data       []byte
	Ascender   float32
	Descender  float32
	LineHeight float32
	Glyphs     []Glyph
	Lut        []int // Hash lookup
	Fallbacks  []int

	sfnt *opentype.Font
}

// State represents the current drawing state.
type State struct {
	Font    int
	Align   int
	Size    float32
	Color   uint32
	Blur    float32
	Spacing float32
}

// FontStash is the main context.
type FontStash struct {
	Params Params

	// Atlas
	Atlas *Atlas

	// Fonts
	Fonts []*Font

	// Texture
	TexData []byte
	Width   int
	Height  int
	Itw     float32 // 1/width
	Ith     float32 // 1/height
	Dirty   image.Rectangle

	// Drawing
	Verts   []float32
	TCoords []float32
	Colors  []uint32
	NVerts  int

	// Scratch buffer (maybe not needed in Go, but useful for caching rasterized glyphs before packing)
	Scratch []byte

	// State stack
	States []State
}

// Params configures the FontStash.
type Params struct {
	Width, Height int
	Renderer      Renderer
	ErrorCallback func(error)
	Flags         int
}

// Alignment flags
const (
	AlignLeft     = 1 << 0
	AlignCenter   = 1 << 1
	AlignRight    = 1 << 2
	AlignTop      = 1 << 3
	AlignMiddle   = 1 << 4
	AlignBottom   = 1 << 5
	AlignBaseline = 1 << 6
)

// Zero coordinate system
const (
	ZeroTopLeft    = 1
	ZeroBottomLeft = 2
)

// Internal limits and defaults
const (
	maxStates         = 20
	maxBlur           = 20
	minFontSize       = 2
	blurPadding       = 2
	initAtlasNodes    = 256
	initFonts         = 4
	maxVertices       = 1024
	whiteRectSize     = 2
	sizeScale         = 10.0
	vertsPerQuad      = 6
)

// Common errors
type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrAtlasFull       = Error("font atlas is full")
	ErrScratchFull     = Error("scratch memory full")
	ErrStatesOverflow  = Error("state stack overflow")
	ErrStatesUnderflow = Error("state stack underflow")
)

// New creates a new FontStash context.
func New(params Params) (*FontStash, error) {
	if params.Width == 0 {
		params.Width = 512
	}
	if params.Height == 0 {
		params.Height = 512
	}

	fs := &FontStash{
		Params:  params,
		Width:   params.Width,
		Height:  params.Height,
		Itw:     1.0 / float32(params.Width),
		Ith:     1.0 / float32(params.Height),
		Dirty:   image.Rectangle{Min: image.Point{params.Width, params.Height}, Max: image.Point{0, 0}},
		Atlas:   newAtlas(params.Width, params.Height, initAtlasNodes), // FONS_INIT_ATLAS_NODES
		Fonts:   make([]*Font, 0, initFonts),
		TexData: make([]byte, params.Width*params.Height),
		States:  make([]State, 0, maxStates),
	}

	// Add white rect at 0,0 for debug drawing.
	fs.addWhiteRect(whiteRectSize, whiteRectSize)

	fs.PushState()
	fs.ClearState()

	return fs, nil
}

func (fs *FontStash) PushState() {
	if len(fs.States) >= maxStates { // FONS_MAX_STATES
		if fs.Params.ErrorCallback != nil {
			fs.Params.ErrorCallback(ErrStatesOverflow)
		}
		return
	}

	if len(fs.States) > 0 {
		top := fs.States[len(fs.States)-1]
		fs.States = append(fs.States, top)
	} else {
		fs.States = append(fs.States, State{}) // Should be cleared anyway
	}
}

func (fs *FontStash) PopState() {
	if len(fs.States) <= 1 {
		if fs.Params.ErrorCallback != nil {
			fs.Params.ErrorCallback(ErrStatesUnderflow)
		}
		return
	}
	fs.States = fs.States[:len(fs.States)-1]
}

func (fs *FontStash) ClearState() {
	state := &fs.States[len(fs.States)-1]
	state.Size = 12.0
	state.Color = 0xffffffff
	state.Font = 0
	state.Blur = 0
	state.Spacing = 0
	state.Align = AlignLeft | AlignBaseline
}

func (fs *FontStash) getState() *State {
	return &fs.States[len(fs.States)-1]
}

func (fs *FontStash) addWhiteRect(w, h int) {
	gx, gy, ok := fs.Atlas.addRect(w, h)
	if !ok {
		return
	}

	// Rasterize
	dst := fs.TexData
	width := fs.Params.Width
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst[(gy+y)*width+(gx+x)] = 0xff
		}
	}

	// Update dirty rect
	if gx < fs.Dirty.Min.X {
		fs.Dirty.Min.X = gx
	}
	if gy < fs.Dirty.Min.Y {
		fs.Dirty.Min.Y = gy
	}
	if gx+w > fs.Dirty.Max.X {
		fs.Dirty.Max.X = gx + w
	}
	if gy+h > fs.Dirty.Max.Y {
		fs.Dirty.Max.Y = gy + h
	}
}

func hashInt(a int) int {
	a += ^(a << 15)
	a ^= (a >> 10)
	a += (a << 3)
	a ^= (a >> 6)
	a += ^(a << 11)
	a ^= (a >> 16)
	return a
}

func (fs *FontStash) getGlyph(f *Font, codepoint rune, isize, iblur int16) (*Glyph, error) {
  if isize < minFontSize { return nil, nil }
  if iblur > maxBlur { iblur = maxBlur }
  pad := int(iblur) + blurPadding

    h := hashInt(int(codepoint)) & (len(f.Lut) - 1)
    i := f.Lut[h]
    for i != -1 {
        g := &f.Glyphs[i]
        if g.Codepoint == codepoint && g.Size == isize && g.Blur == iblur {
            return g, nil
        }
        i = g.Next
    }

    // Create glyph
    gIndex := fs.getGlyphIndex(f, codepoint)
    renderFont := f
    if gIndex == 0 {
        for _, fb := range f.Fallbacks {
            fallbackFont := fs.Fonts[fb]
            fallbackIndex := fs.getGlyphIndex(fallbackFont, codepoint)
            if fallbackIndex != 0 {
                gIndex = fallbackIndex
                renderFont = fallbackFont
                break
            }
        }
    }

    size := float64(isize) / sizeScale

    // Get glyph metrics and bitmap
    face, err := opentype.NewFace(renderFont.sfnt, &opentype.FaceOptions{
        Size: size,
        DPI: 72,
        Hinting: font.HintingFull,
    })
    if err != nil { return nil, err }
    defer face.Close()

    // Bounds check skipped
    _, advance, ok := face.GlyphBounds(codepoint)
    if !ok {
        // Continue but with empty bounds/image?
        // Fallthrough
    }

    dr, mask, _, _, ok := face.Glyph(fixed.P(0, 0), codepoint)
    if !ok {
        // Fallthrough
    }

    gw := dr.Dx() + pad*2
    gh := dr.Dy() + pad*2

    // Find free spot
    gx, gy, ok := fs.Atlas.addRect(gw, gh)
    if !ok {
        // Atlas full
        if fs.Params.ErrorCallback != nil {
            fs.Params.ErrorCallback(ErrAtlasFull)
        }
        // Try again? The C code calls handler and tries again.
        // User might resize in callback.
        gx, gy, ok = fs.Atlas.addRect(gw, gh)
        if !ok {
            return nil, ErrAtlasFull
        }
    }

    // Init glyph
    glyph := Glyph{
        Codepoint: codepoint,
        Size:      isize,
        Blur:      iblur,
        Index:     gIndex,
        X0:        int16(gx),
        Y0:        int16(gy),
        X1:        int16(gx + gw),
        Y1:        int16(gy + gh),
        XAdv:      int16(int32(advance) * sizeScale / 64),
        XOff: int16(dr.Min.X - pad),
        YOff: int16(dr.Min.Y - pad),
    }

    // Copy bitmap to texture
    dst := fs.TexData
    width := fs.Params.Width

    if mask != nil {
        b := mask.Bounds()
        for y := 0; y < b.Dy(); y++ {
            for x := 0; x < b.Dx(); x++ {
                 _, _, _, a := mask.At(x + b.Min.X, y + b.Min.Y).RGBA()
                 val := uint8(a >> 8)

                 targetX := gx + pad + x
                 targetY := gy + pad + y
                 if targetX < width && targetY < fs.Params.Height {
                     dst[targetY * width + targetX] = val
                 }
            }
        }
    }

    // Blur if needed
    if iblur > 0 {
        fs.blur(gx, gy, gw, gh, width, int(iblur))
    }

    // Update dirty rect
    if gx < fs.Dirty.Min.X { fs.Dirty.Min.X = gx }
    if gy < fs.Dirty.Min.Y { fs.Dirty.Min.Y = gy }
    if gx+gw > fs.Dirty.Max.X { fs.Dirty.Max.X = gx+gw }
    if gy+gh > fs.Dirty.Max.Y { fs.Dirty.Max.Y = gy+gh }

    // Add to cache
    f.Glyphs = append(f.Glyphs, glyph)
    f.Glyphs[len(f.Glyphs)-1].Next = f.Lut[h]
    f.Lut[h] = len(f.Glyphs) - 1

    return &f.Glyphs[len(f.Glyphs)-1], nil
}

func (fs *FontStash) blur(x, y, w, h, stride, blur int) {
	if blur < 1 {
		return
	}

	sigma := float32(blur) * 0.57735 // 1 / sqrt(3)
	alpha := int((1 << 16) * (1.0 - math.Exp(float64(-2.3/(sigma+1.0)))))

	fs.blurRows(x, y, w, h, stride, alpha)
	fs.blurCols(x, y, w, h, stride, alpha)
	fs.blurRows(x, y, w, h, stride, alpha)
	fs.blurCols(x, y, w, h, stride, alpha)
}

func (fs *FontStash) blurRows(x, y, w, h, stride, alpha int) {
	dst := fs.TexData
	for r := 0; r < h; r++ {
		offset := (y+r)*stride + x
		z := 0
		for c := 1; c < w; c++ {
			z += (alpha * ((int(dst[offset+c]) << 7) - z)) >> 16
			dst[offset+c] = uint8(z >> 7)
		}
		dst[offset+w-1] = 0
		z = 0
		for c := w - 2; c >= 0; c-- {
			z += (alpha * ((int(dst[offset+c]) << 7) - z)) >> 16
			dst[offset+c] = uint8(z >> 7)
		}
		dst[offset] = 0
	}
}

// SetSize sets the font size in the current state.
func (fs *FontStash) SetSize(size float32) {
	fs.getState().Size = size
}

// SetColor sets the color in the current state.
func (fs *FontStash) SetColor(color uint32) {
	fs.getState().Color = color
}

// SetSpacing sets the character spacing in the current state.
func (fs *FontStash) SetSpacing(spacing float32) {
	fs.getState().Spacing = spacing
}

// SetBlur sets the blur amount in the current state.
func (fs *FontStash) SetBlur(blur float32) {
	fs.getState().Blur = blur
}

// SetAlign sets the text alignment in the current state.
func (fs *FontStash) SetAlign(align int) {
	fs.getState().Align = align
}

// SetFont sets the current font.
func (fs *FontStash) SetFont(font int) {
	fs.getState().Font = font
}

func (fs *FontStash) getVertAlign(f *Font, align int, isize int16) float32 {
	size := float32(isize) / sizeScale
	if fs.Params.Flags&ZeroTopLeft != 0 {
		if align&AlignTop != 0 {
			return f.Ascender * size
		} else if align&AlignMiddle != 0 {
			return (f.Ascender + f.Descender) / 2.0 * size
		} else if align&AlignBaseline != 0 {
			return 0.0
		} else if align&AlignBottom != 0 {
			return f.Descender * size
		}
	} else {
		if align&AlignTop != 0 {
			return -f.Ascender * size
		} else if align&AlignMiddle != 0 {
			return -(f.Ascender + f.Descender) / 2.0 * size
		} else if align&AlignBaseline != 0 {
			return 0.0
		} else if align&AlignBottom != 0 {
			return -f.Descender * size
		}
	}
	return 0.0
}

type Quad struct {
	X0, Y0, S0, T0 float32
	X1, Y1, S1, T1 float32
}

func (fs *FontStash) getQuad(f *Font, prevGlyphIndex int, glyph *Glyph, scale, spacing float32, x, y *float32, q *Quad) {
	if prevGlyphIndex != -1 {
		adv := fs.getGlyphKernAdvance(f, prevGlyphIndex, glyph.Index, float32(glyph.Size)/sizeScale)
		*x += float32(int(float32(adv)*scale + spacing + 0.5))
	}

	xoff := float32(glyph.XOff + 1)
	yoff := float32(glyph.YOff + 1)
	x0 := float32(glyph.X0 + 1)
	y0 := float32(glyph.Y0 + 1)
	x1 := float32(glyph.X1 - 1)
	y1 := float32(glyph.Y1 - 1)

	var rx, ry float32
	if fs.Params.Flags&ZeroTopLeft != 0 {
		rx = float32(int(*x + xoff))
		ry = float32(int(*y + yoff))

		q.X0 = rx
		q.Y0 = ry
		q.X1 = rx + x1 - x0
		q.Y1 = ry + y1 - y0

		q.S0 = x0 * fs.Itw
		q.T0 = y0 * fs.Ith
		q.S1 = x1 * fs.Itw
		q.T1 = y1 * fs.Ith
	} else {
		rx = float32(int(*x + xoff))
		ry = float32(int(*y - yoff))

		q.X0 = rx
		q.Y0 = ry
		q.X1 = rx + x1 - x0
		q.Y1 = ry - y1 + y0

		q.S0 = x0 * fs.Itw
		q.T0 = y0 * fs.Ith
		q.S1 = x1 * fs.Itw
		q.T1 = y1 * fs.Ith
	}

	*x += float32(int(float32(glyph.XAdv)/sizeScale + 0.5))
}

func (fs *FontStash) vertex(x, y, s, t float32, c uint32) {
	fs.Verts = append(fs.Verts, x, y)
	fs.TCoords = append(fs.TCoords, s, t)
	fs.Colors = append(fs.Colors, c)
	fs.NVerts++
}

// DrawText draws the text at the specified position.
func (fs *FontStash) DrawText(x, y float32, str string) float32 {
	state := fs.getState()
	if state.Font < 0 || state.Font >= len(fs.Fonts) {
		return x
	}
	f := fs.Fonts[state.Font]
	if f.Data == nil {
		return x
	}

	isize := int16(state.Size * sizeScale)
	iblur := int16(state.Blur)

	scale := float32(1.0)

	if state.Align&AlignLeft != 0 {
		// empty
	} else if state.Align&AlignRight != 0 {
		width := fs.TextBounds(x, y, str, nil)
		x -= width
	} else if state.Align&AlignCenter != 0 {
		width := fs.TextBounds(x, y, str, nil)
		x -= width * 0.5
	}

	y += fs.getVertAlign(f, state.Align, isize)

	q := Quad{}
	prevGlyphIndex := -1

	for _, codepoint := range str {
		glyph, err := fs.getGlyph(f, codepoint, isize, iblur)
		if err != nil {
			continue // Or stop?
		}
		if glyph != nil {
			fs.getQuad(f, prevGlyphIndex, glyph, scale, state.Spacing, &x, &y, &q)

			if fs.NVerts+vertsPerQuad > maxVertices { // FONS_VERTEX_COUNT
				fs.flush()
			}

			fs.vertex(q.X0, q.Y0, q.S0, q.T0, state.Color)
			fs.vertex(q.X1, q.Y1, q.S1, q.T1, state.Color)
			fs.vertex(q.X1, q.Y0, q.S1, q.T0, state.Color)

			fs.vertex(q.X0, q.Y0, q.S0, q.T0, state.Color)
			fs.vertex(q.X0, q.Y1, q.S0, q.T1, state.Color)
			fs.vertex(q.X1, q.Y1, q.S1, q.T1, state.Color)
		}
		if glyph != nil {
			prevGlyphIndex = glyph.Index
		} else {
			prevGlyphIndex = -1
		}
	}
	fs.flush()

	return x
}

// TextBounds measures the text bounds.
func (fs *FontStash) TextBounds(x, y float32, str string, bounds *[4]float32) float32 {
	state := fs.getState()
	if state.Font < 0 || state.Font >= len(fs.Fonts) {
		return 0
	}
	f := fs.Fonts[state.Font]
	isize := int16(state.Size * sizeScale)
	iblur := int16(state.Blur)
	scale := float32(1.0)

	y += fs.getVertAlign(f, state.Align, isize)

	minx, maxx := x, x
	miny, maxy := y, y
	startx := x

	q := Quad{}
	prevGlyphIndex := -1

	for _, codepoint := range str {
		glyph, err := fs.getGlyph(f, codepoint, isize, iblur)
		if err != nil {
			continue
		}
		if glyph != nil {
			fs.getQuad(f, prevGlyphIndex, glyph, scale, state.Spacing, &x, &y, &q)
			if q.X0 < minx {
				minx = q.X0
			}
			if q.X1 > maxx {
				maxx = q.X1
			}
			if fs.Params.Flags&ZeroTopLeft != 0 {
				if q.Y0 < miny {
					miny = q.Y0
				}
				if q.Y1 > maxy {
					maxy = q.Y1
				}
			} else {
				if q.Y1 < miny {
					miny = q.Y1
				}
				if q.Y0 > maxy {
					maxy = q.Y0
				}
			}
		}
		if glyph != nil {
			prevGlyphIndex = glyph.Index
		} else {
			prevGlyphIndex = -1
		}
	}

	advance := x - startx

	if state.Align&AlignLeft != 0 {
		// empty
	} else if state.Align&AlignRight != 0 {
		minx -= advance
		maxx -= advance
	} else if state.Align&AlignCenter != 0 {
		minx -= advance * 0.5
		maxx -= advance * 0.5
	}

	if bounds != nil {
		bounds[0] = minx
		bounds[1] = miny
		bounds[2] = maxx
		bounds[3] = maxy
	}

	return advance
}

// VertMetrics returns the vertical metrics for the current font.
func (fs *FontStash) VertMetrics() (ascender, descender, lineHeight float32) {
	state := fs.getState()
	if state.Font < 0 || state.Font >= len(fs.Fonts) {
		return 0, 0, 0
	}
	f := fs.Fonts[state.Font]
	size := state.Size

	return f.Ascender * size, f.Descender * size, f.LineHeight * size
}

// LineBounds returns the vertical bounds for the current font at the given line position.
func (fs *FontStash) LineBounds(y float32) (miny, maxy float32) {
	state := fs.getState()
	if state.Font < 0 || state.Font >= len(fs.Fonts) {
		return y, y
	}
	f := fs.Fonts[state.Font]
	isize := int16(state.Size * sizeScale)
	size := state.Size

	y += fs.getVertAlign(f, state.Align, isize)

	if fs.Params.Flags&ZeroTopLeft != 0 {
		miny = y - f.Ascender*size
		maxy = miny + f.LineHeight*size
	} else {
		maxy = y + f.Descender*size
		miny = maxy - f.LineHeight*size
	}
	return
}

// ExpandAtlas expands the font atlas to the given dimensions.
func (fs *FontStash) ExpandAtlas(width, height int) bool {
	width = maxInt(width, fs.Params.Width)
	height = maxInt(height, fs.Params.Height)

	if width == fs.Params.Width && height == fs.Params.Height {
		return true
	}

	// Flush pending glyphs
	fs.flush()

	// Create new texture in renderer
	if fs.Params.Renderer != nil {
		fs.Params.Renderer.Resize(width, height)
	}

	// Copy old texture data
	newTexData := make([]byte, width*height)
	for i := 0; i < fs.Params.Height; i++ {
		src := fs.TexData[i*fs.Params.Width : i*fs.Params.Width+fs.Params.Width]
		dst := newTexData[i*width : i*width+fs.Params.Width]
		copy(dst, src)
	}

	fs.TexData = newTexData

	// Increase atlas size
	fs.Atlas.expand(width, height)

	// Add existing data as dirty
	maxy := 0
	for _, n := range fs.Atlas.nodes {
		if int(n.y) > maxy {
			maxy = int(n.y)
		}
	}
	// Dirty rect logic: inverted init, then expand to cover valid area
	// Here we expand to cover the existing valid nodes.
	// But effectively the dirty rect should cover the valid area if we copied it.
	// Actually, ExpandAtlas copies old data to new buffer.
	// So we might want to mark the copied area as dirty? Or not?
	// The renderer's Resize might clear texture or keep it.
	// If renderer keeps content on Resize, we don't need to re-upload everything?
	// But generic renderer might lose content.
	// The safest is to mark everything dirty.
	fs.Dirty = image.Rect(0, 0, width, maxy)

	fs.Params.Width = width
	fs.Params.Height = height
	fs.Width = width
	fs.Height = height
	fs.Itw = 1.0 / float32(width)
	fs.Ith = 1.0 / float32(height)

	return true
}

// ResetAtlas resets the atlas to the given dimensions.
func (fs *FontStash) ResetAtlas(width, height int) bool {
	// Flush pending glyphs
	fs.flush()

	// Create new texture in renderer
	if fs.Params.Renderer != nil {
		fs.Params.Renderer.Resize(width, height)
	}

	// Reset atlas
	fs.Atlas.reset(width, height)

	// Clear texture data
	fs.TexData = make([]byte, width*height)

	// Reset dirty rect
	fs.Dirty = image.Rectangle{Min: image.Point{width, height}, Max: image.Point{0, 0}}

	// Reset cached glyphs
	for _, font := range fs.Fonts {
		font.Glyphs = font.Glyphs[:0]
		for i := range font.Lut {
			font.Lut[i] = -1
		}
	}

	fs.Params.Width = width
	fs.Params.Height = height
	fs.Width = width
	fs.Height = height
	fs.Itw = 1.0 / float32(width)
	fs.Ith = 1.0 / float32(height)

	// Add white rect
	fs.addWhiteRect(whiteRectSize, whiteRectSize)

	return true
}

func (fs *FontStash) flush() {
	// Flush texture
	if fs.Dirty.Min.X < fs.Dirty.Max.X && fs.Dirty.Min.Y < fs.Dirty.Max.Y {
		if fs.Params.Renderer != nil {
			// Check bounds?
			// The dirty rect might be larger than texture?
			// fs.Dirty is in texture coords.
			fs.Params.Renderer.Update(fs.Dirty, fs.TexData, fs.Params.Width)
		}
		// Reset dirty rect
		fs.Dirty = image.Rectangle{Min: image.Point{fs.Params.Width, fs.Params.Height}, Max: image.Point{0, 0}}
	}

	// Flush triangles
	if fs.NVerts > 0 {
		if fs.Params.Renderer != nil {
			// Convert fs.Verts, fs.TCoords, fs.Colors to []Vertex
			verts := make([]Vertex, fs.NVerts)
			for i := 0; i < fs.NVerts; i++ {
				verts[i] = Vertex{
					X:     fs.Verts[i*2],
					Y:     fs.Verts[i*2+1],
					U:     fs.TCoords[i*2],
					V:     fs.TCoords[i*2+1],
					Color: fs.Colors[i],
				}
			}
			fs.Params.Renderer.Draw(verts)
		}
		fs.NVerts = 0
		fs.Verts = fs.Verts[:0]
		fs.TCoords = fs.TCoords[:0]
		fs.Colors = fs.Colors[:0]
	}
}

func (fs *FontStash) blurCols(x, y, w, h, stride, alpha int) {
	dst := fs.TexData
	for c := 0; c < w; c++ {
		offset := y*stride + x + c
		z := 0
		for r := stride; r < h*stride; r += stride {
			z += (alpha * ((int(dst[offset+r]) << 7) - z)) >> 16
			dst[offset+r] = uint8(z >> 7)
		}
		dst[offset+(h-1)*stride] = 0
		z = 0
		for r := (h - 2) * stride; r >= 0; r -= stride {
			z += (alpha * ((int(dst[offset+r]) << 7) - z)) >> 16
			dst[offset+r] = uint8(z >> 7)
		}
		dst[offset] = 0
	}
}
