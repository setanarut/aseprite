package aseprite

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
)

func expandSliceKey(slice *Slice, lenFrames int, frameIndices []int) {
	if len(slice.Frames) == lenFrames {
		return
	}
	expandedKeys := make([]SliceFrame, lenFrames)
	keyIdx := 0
	var current SliceFrame
	if len(slice.Frames) > 0 {
		current = slice.Frames[0]
	}
	for frameIdx := range expandedKeys {
		if keyIdx < len(slice.Frames) && frameIndices[keyIdx] == frameIdx {
			current = slice.Frames[keyIdx]
			keyIdx++
		}
		expandedKeys[frameIdx] = current
	}
	slice.Frames = expandedKeys
}
func skipString(raw []byte) []byte {
	n := binary.LittleEndian.Uint16(raw)
	return raw[2+n:]
}
func parseString(raw []byte) string {
	n := binary.LittleEndian.Uint16(raw)
	return string(raw[2 : 2+n])
}
func parseColor(raw []byte) color.NRGBA {
	return color.NRGBA{
		R: raw[0],
		G: raw[1],
		B: raw[2],
		A: raw[3],
	}
}

func drawTile(dst draw.Image, src image.Image, x, y int, flags FlipBitMask) {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	for dy := range h {
		for dx := range w {
			sx, sy := dx, dy
			if flags.IsFlipX() {
				sx = w - 1 - sx
			}
			if flags.IsFlipY() {
				sy = h - 1 - sy
			}
			if flags.IsFlipD() {
				sx, sy = sy, sx
			}
			c := src.At(bounds.Min.X+sx, bounds.Min.Y+sy)
			dst.Set(x+dx, y+dy, c)
		}
	}
}
