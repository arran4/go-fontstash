package fontstash

type atlasNode struct {
	x, y, width int16
}

type Atlas struct {
	width, height int
	nodes         []atlasNode
}

func newAtlas(w, h, nnodes int) *Atlas {
	a := &Atlas{
		width:  w,
		height: h,
		nodes:  make([]atlasNode, 0, nnodes),
	}

	// Init root node.
	a.nodes = append(a.nodes, atlasNode{
		x:     0,
		y:     0,
		width: int16(w),
	})

	return a
}

func (a *Atlas) insertNode(idx, x, y, w int) {
	// Insert node
	// In Go, we can use slice manipulation
	node := atlasNode{
		x:     int16(x),
		y:     int16(y),
		width: int16(w),
	}

	a.nodes = append(a.nodes, atlasNode{}) // Grow slice
	copy(a.nodes[idx+1:], a.nodes[idx:])
	a.nodes[idx] = node
}

func (a *Atlas) removeNode(idx int) {
	if len(a.nodes) == 0 {
		return
	}
	a.nodes = append(a.nodes[:idx], a.nodes[idx+1:]...)
}

func (a *Atlas) expand(w, h int) {
	// Insert node for empty space
	if w > a.width {
		a.insertNode(len(a.nodes), a.width, 0, w-a.width)
	}
	a.width = w
	a.height = h
}

func (a *Atlas) reset(w, h int) {
	a.width = w
	a.height = h
	a.nodes = a.nodes[:0]

	// Init root node.
	a.nodes = append(a.nodes, atlasNode{
		x:     0,
		y:     0,
		width: int16(w),
	})
}

func (a *Atlas) addSkylineLevel(idx, x, y, w, h int) bool {
	// Insert new node
	a.insertNode(idx, x, y+h, w)

	// Delete skyline segments that fall under the shadow of the new segment.
	for i := idx + 1; i < len(a.nodes); i++ {
		prev := a.nodes[i-1]
		curr := a.nodes[i]

		if curr.x < prev.x+prev.width {
			shrink := int(prev.x + prev.width - curr.x)
			a.nodes[i].x += int16(shrink)
			a.nodes[i].width -= int16(shrink)

			if a.nodes[i].width <= 0 {
				a.removeNode(i)
				i--
			} else {
				break
			}
		} else {
			break
		}
	}

	// Merge same height skyline segments that are next to each other.
	for i := 0; i < len(a.nodes)-1; i++ {
		if a.nodes[i].y == a.nodes[i+1].y {
			a.nodes[i].width += a.nodes[i+1].width
			a.removeNode(i + 1)
			i--
		}
	}

	return true
}

func (a *Atlas) rectFits(i, w, h int) int {
	// Checks if there is enough space at the location of skyline span 'i',
	// and return the max height of all skyline spans under that at that location,
	// (think tetris block being dropped at that position). Or -1 if no space found.
	x := int(a.nodes[i].x)
	y := int(a.nodes[i].y)

	if x+w > a.width {
		return -1
	}

	spaceLeft := w
	for spaceLeft > 0 {
		if i == len(a.nodes) {
			return -1
		}
		y = maxInt(y, int(a.nodes[i].y))
		if y+h > a.height {
			return -1
		}
		spaceLeft -= int(a.nodes[i].width)
		i++
	}
	return y
}

func (a *Atlas) addRect(rw, rh int) (rx, ry int, ok bool) {
	besth := a.height
	bestw := a.width
	besti := -1
	bestx := -1
	besty := -1

	// Bottom left fit heuristic.
	for i := 0; i < len(a.nodes); i++ {
		y := a.rectFits(i, rw, rh)
		if y != -1 {
			// Score based on height and then width
			if y+rh < besth || (y+rh == besth && int(a.nodes[i].width) < bestw) {
				besti = i
				bestw = int(a.nodes[i].width)
				besth = y + rh
				bestx = int(a.nodes[i].x)
				besty = y
			}
		}
	}

	if besti == -1 {
		return 0, 0, false
	}

	// Perform the actual packing.
	if !a.addSkylineLevel(besti, bestx, besty, rw, rh) {
		return 0, 0, false
	}

	return bestx, besty, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
