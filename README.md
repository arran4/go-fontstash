# Font Stash

Font Stash is a light-weight online font texture atlas builder written in Go. It uses `golang.org/x/image/font` to render fonts on demand to a texture atlas.

## Example

```go
package main

import (
	"fmt"
	"image"
	"log"

	"github.com/yourname/fontstash/fontstash"
)

type MyRenderer struct {
	// ... backend specific fields (e.g. OpenGL texture ID)
}

func (r *MyRenderer) Resize(width, height int) {
	fmt.Printf("Resize atlas to %dx%d\n", width, height)
	// Create or resize texture
}

func (r *MyRenderer) Update(rect image.Rectangle, data []byte, imgWidth int) {
	fmt.Printf("Update atlas rect %v\n", rect)
	// Update texture subimage
}

func (r *MyRenderer) Draw(verts []fontstash.Vertex) {
	fmt.Printf("Draw %d vertices\n", len(verts))
	// Render triangles
}

func main() {
	fs, err := fontstash.New(fontstash.Params{
		Width:  512,
		Height: 512,
		Renderer: &MyRenderer{},
	})
	if err != nil {
		log.Fatal(err)
	}

	fontNormal, err := fs.AddFont("sans", "DroidSerif-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}

	fs.SetFont(fontNormal)
	fs.SetSize(24.0)
	fs.SetColor(0xffffffff)

	fs.DrawText(10, 10, "Hello Font Stash!")
}
```

## License
The library is licensed under [zlib license](LICENSE.txt).
