// Aseprite file parser/decoder
package aseprite

//go:generate stringer -type=LayerType,BlendMode,ColorDepth,LoopDirection,CelType,FlipBitMask -output=type_string.go

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"io/fs"
	"math"
	"os"
	"time"
)

type BlendMode uint16
type LayerType uint16
type ColorDepth uint16
type CelType uint16
type LoopDirection uint8
type FlipBitMask uint8

const (
	//Bitmask for X flip
	FlipX FlipBitMask = 1 << iota
	// Bitmask for Y flip
	FlipY
	// Bitmask for diagonal flip (swap X/Y axis)
	FlipD
)

// is Y flip
func (f FlipBitMask) IsFlipX() bool {
	return f&FlipX != 0
}

// is X flip
func (f FlipBitMask) IsFlipY() bool {
	return f&FlipY != 0
}

// is diagonal flip (swap X/Y axis)
func (f FlipBitMask) IsFlipD() bool {
	return f&FlipD != 0
}

const (
	NRGBA ColorDepth = 32
	// Grayscale mode is parsed as image.NRGBA images.
	Grayscale ColorDepth = 16
	Indexed   ColorDepth = 8
)

const (
	Forward LoopDirection = iota
	Reverse
	PingPong
	PingPongReverse
)

const (
	Image LayerType = iota
	Group
	Tilemap
)

const (
	RawUncompressed CelType = iota
	LinkedCel
	CompressedImage
	CompressedTilemap
)

const (
	Normal BlendMode = iota
	Multiply
	Screen
	Overlay
	Darken
	Lighten
	ColorDodge
	ColorBurn
	HardLight
	SoftLight
	Difference
	Exclusion
	Hue
	Saturation
	Color
	Luminosity
	Addition
	Subtract
	Divide
)

type userDataReceiver interface {
	setUserData(text string, c color.NRGBA)
}

type Ase struct {
	ColorDepth ColorDepth

	// Canvas size (width and height)
	Size image.Point
	// Timeline frames containing layer Cels
	Frames []Frame
	// Layer datas. Layer indices increase from bottom to top.
	Layers []*Layer
	// Aseprite slices. https://www.aseprite.org/api/slice#slice
	Slices []Slice
	// Timeline animation tags
	Tags []Tag

	// Palette index for used as transparent color in each layer. (only for indexed images)
	TransparentIndex uint8
	// Palette of file. (only for indexed images)
	Palette color.Palette

	// Tilesets used in Tilemap layers
	Tilesets []*Tileset
}

// Tileset represents a set of tiles defined in the .ase file (Chunk 0x2023)
type Tileset struct {
	ID        uint32
	Flags     uint32
	NumTiles  int
	BaseIndex int // Index to show in the UI (usually 1)
	TileSize  image.Point
	Name      string
	// External file data (Present if Flags & 1 is set)
	ExternalFileID    uint32
	ExternalTilesetID uint32
	// Tileset image containing all tiles vertically (Present if Flags & 2 is set)
	Image image.Image
}

func (a *Ase) parse(r io.Reader, onlyVisible bool) (filesize int64, err error) {
	var hdr [128]byte
	if n, err := io.ReadFull(r, hdr[:]); err != nil {
		return int64(n), err
	}
	if magic := binary.LittleEndian.Uint16(hdr[4:]); magic != 0xA5E0 {
		return 128, errors.New("invalid magic number")
	}
	if pixw, pixh := hdr[34], hdr[35]; pixw != pixh {
		return 128, errors.New("unsupported pixel ratio")
	}
	fileSize := int64(binary.LittleEndian.Uint32(hdr[:]))
	totalFrames := int(binary.LittleEndian.Uint16(hdr[6:]))
	a.Size.X = int(binary.LittleEndian.Uint16(hdr[8:]))
	a.Size.Y = int(binary.LittleEndian.Uint16(hdr[10:]))
	a.ColorDepth = ColorDepth(binary.LittleEndian.Uint16(hdr[12:]))
	a.TransparentIndex = hdr[28]
	paletteSize := binary.LittleEndian.Uint16(hdr[32:])
	a.Palette = make(color.Palette, paletteSize)
	a.Frames = make([]Frame, 0, totalFrames)

	for i := range a.Palette {
		a.Palette[i] = color.Black
	}
	a.Palette[a.TransparentIndex] = color.Transparent
	chunkHeaderBuf := make([]byte, 6)
	frameHeaderBuf := make([]byte, 16)
	currentOffset := int64(128)

	for range totalFrames {
		if _, err := io.ReadFull(r, frameHeaderBuf); err != nil {
			return currentOffset, err
		}
		currentOffset += 16
		if magic := binary.LittleEndian.Uint16(frameHeaderBuf[4:]); magic != 0xF1FA {
			return currentOffset, errors.New("invalid frame magic number")
		}
		oldChunks := binary.LittleEndian.Uint16(frameHeaderBuf[6:])
		durationMS := binary.LittleEndian.Uint16(frameHeaderBuf[8:])
		newChunks := binary.LittleEndian.Uint32(frameHeaderBuf[12:])
		nchunks := int(newChunks)
		if nchunks == 0 {
			nchunks = int(oldChunks)
		}
		currentFrame := Frame{
			Duration: time.Millisecond * time.Duration(durationMS),
		}
		if len(a.Layers) > 0 {
			currentFrame.Cels = make([]*Cel, len(a.Layers))
		}
		var lastUserDataTarget userDataReceiver
		var tagQueue []userDataReceiver
		for j := 0; j < nchunks; j++ {
			if _, err := io.ReadFull(r, chunkHeaderBuf); err != nil {
				return currentOffset, err
			}
			currentOffset += 6
			chunkSize := binary.LittleEndian.Uint32(chunkHeaderBuf[0:])
			chunkType := binary.LittleEndian.Uint16(chunkHeaderBuf[4:])
			if chunkType != 0x2020 {
				tagQueue = nil
			}
			dataLen := int(chunkSize) - 6
			if dataLen < 0 {
				return currentOffset, errors.New("invalid chunk size")
			}
			chunkData := make([]byte, dataLen)
			if _, err := io.ReadFull(r, chunkData); err != nil {
				return currentOffset, err
			}
			currentOffset += int64(dataLen)
			switch chunkType {
			case 0x2004:
				l := a.parseLayer(chunkData)
				a.Layers = append(a.Layers, &l)
				lastUserDataTarget = a.Layers[len(a.Layers)-1]
				if len(currentFrame.Cels) < len(a.Layers) {
					newCels := make([]*Cel, len(a.Layers))
					copy(newCels, currentFrame.Cels)
					currentFrame.Cels = newCels
				}
			case 0x2005:
				layerIdx := int(binary.LittleEndian.Uint16(chunkData))
				if layerIdx >= len(currentFrame.Cels) {
					newSize := max(len(a.Layers), layerIdx+1)
					newCels := make([]*Cel, newSize)
					copy(newCels, currentFrame.Cels)
					currentFrame.Cels = newCels
				}
				cel, err := a.parseCel(chunkData, layerIdx)
				if err != nil {
					return currentOffset, err
				}
				if cel != nil {
					currentFrame.Cels[layerIdx] = cel
					lastUserDataTarget = currentFrame.Cels[layerIdx]
				}
			case 0x2019:
				a.parsePalette(chunkData)
				lastUserDataTarget = nil
			case 0x0004:
				a.parseOldPalette0x0004(chunkData)
				lastUserDataTarget = nil
			case 0x0011:
				a.parseOldPalette0x0011(chunkData)
				lastUserDataTarget = nil
			case 0x2018:
				newTags := a.parseTags(chunkData)
				startIdx := len(a.Tags)
				a.Tags = append(a.Tags, newTags...)
				for i := range newTags {
					tagQueue = append(tagQueue, &a.Tags[startIdx+i])
				}
				lastUserDataTarget = nil
			case 0x2022:
				slice := a.parseSlice(chunkData, totalFrames)
				a.Slices = append(a.Slices, slice)
				lastUserDataTarget = &a.Slices[len(a.Slices)-1]
			case 0x2023:
				ts, err := a.parseTileset(chunkData)
				if err != nil {
					return currentOffset, err
				}
				a.Tilesets = append(a.Tilesets, &ts)
			case 0x2020:
				var target userDataReceiver
				if len(tagQueue) > 0 {
					target = tagQueue[0]
					tagQueue = tagQueue[1:]
				} else {
					target = lastUserDataTarget
				}
				if target != nil {
					data, col := a.parseUserData(chunkData)
					target.setUserData(string(data), col)
				}
				lastUserDataTarget = nil
			default:
				lastUserDataTarget = nil
			}
		}
		if len(currentFrame.Cels) < len(a.Layers) {
			newCels := make([]*Cel, len(a.Layers))
			copy(newCels, currentFrame.Cels)
			currentFrame.Cels = newCels
		}
		a.Frames = append(a.Frames, currentFrame)
	}

	keptLayers := make([]*Layer, 0, len(a.Layers))
	keptLayerIndices := make(map[int]struct{})

	// Map to track the visibility of each level.
	// levelVisibility[0] = true means Level 0 (root) is visible.
	levelVisibility := make(map[uint16]bool)

	for i, l := range a.Layers {
		// Ignore technical layers
		// if l.Type == Tilemap {
		// 	continue
		// }

		// Determine the "Calculated" visibility of the layer.
		// For a layer to be visible:
		// 1. Its own flag must be visible (l.Flags.IsVisible)
		// 2. If it is a child (Level > 0), the parent level (Level-1) must be visible.
		parentVisible := true
		if l.ChildLevel > 0 {
			if visible, ok := levelVisibility[l.ChildLevel-1]; ok {
				parentVisible = visible
			}
		}

		isEffectiveVisible := l.IsVisible && parentVisible

		// Save the state of this layer to the map.
		// If this is a group, subsequent children (Level+1) will read this value.
		levelVisibility[l.ChildLevel] = isEffectiveVisible

		if onlyVisible && !isEffectiveVisible {
			continue
		}

		keptLayers = append(keptLayers, l)
		keptLayerIndices[i] = struct{}{}
	}
	a.Layers = keptLayers

	for i := range a.Frames {
		frame := &a.Frames[i]
		newCels := make([]*Cel, 0, len(keptLayers))
		for j, cel := range frame.Cels {
			if _, ok := keptLayerIndices[j]; ok {
				newCels = append(newCels, cel)
			}
		}
		frame.Cels = newCels
	}

	for _, lyr := range a.Layers {
		if lyr.Type == Tilemap {
			lyr.Tileset = a.Tilesets[lyr.TilesetIndex]
		}
	}

	return fileSize, nil
}

// Chunk0x2023
func (a *Ase) parseTileset(raw []byte) (Tileset, error) {
	var ts Tileset
	if len(raw) < 32 {
		return ts, errors.New("tileset chunk too small")
	}

	ts.ID = binary.LittleEndian.Uint32(raw[0:])
	ts.Flags = binary.LittleEndian.Uint32(raw[4:])
	ts.NumTiles = int(binary.LittleEndian.Uint32(raw[8:]))
	ts.TileSize.X = int(binary.LittleEndian.Uint16(raw[12:]))
	ts.TileSize.Y = int(binary.LittleEndian.Uint16(raw[14:]))
	ts.BaseIndex = int(int16(binary.LittleEndian.Uint16(raw[16:])))

	// 14 bytes reserved: raw[18:32] skipped

	raw = raw[32:]

	// Parse Name
	ts.Name = parseString(raw)
	nameLen := binary.LittleEndian.Uint16(raw)
	raw = raw[2+nameLen:]

	// External File (Flag 1)
	if ts.Flags&1 != 0 {
		if len(raw) < 8 {
			return ts, errors.New("invalid tileset external info")
		}
		ts.ExternalFileID = binary.LittleEndian.Uint32(raw[0:])
		ts.ExternalTilesetID = binary.LittleEndian.Uint32(raw[4:])
		raw = raw[8:]
	}

	// Compressed Tileset Image (Flag 2)
	if ts.Flags&2 != 0 {
		if len(raw) < 4 {
			return ts, errors.New("invalid tileset image length")
		}
		raw = raw[4:]

		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return ts, err
		}
		defer zr.Close()

		pix, err := io.ReadAll(zr)
		if err != nil {
			return ts, err
		}

		// Tileset image rect
		tilesetBounds := image.Rect(0, 0, ts.TileSize.X, ts.TileSize.Y*ts.NumTiles)

		switch a.ColorDepth {
		case Indexed:
			ts.Image = &image.Paletted{
				Pix:     pix,
				Stride:  tilesetBounds.Dx(),
				Rect:    tilesetBounds,
				Palette: a.Palette,
			}
		case Grayscale:
			nrgba := image.NewNRGBA(tilesetBounds)
			stride := tilesetBounds.Dx() * 2
			for y := 0; y < tilesetBounds.Dy(); y++ {
				for x := 0; x < tilesetBounds.Dx(); x++ {
					i := y*stride + x*2
					if i+1 < len(pix) {
						grayValue := pix[i]
						alphaValue := pix[i+1]
						nrgba.SetNRGBA(x, y, color.NRGBA{
							R: grayValue,
							G: grayValue,
							B: grayValue,
							A: alphaValue,
						})
					}
				}
			}
			ts.Image = nrgba
		case NRGBA:
			ts.Image = &image.NRGBA{
				Pix:    pix,
				Stride: tilesetBounds.Dx() * 4,
				Rect:   tilesetBounds,
			}
		default:
			return ts, errors.New("invalid color depth for tileset")
		}
	}

	return ts, nil
}

// Chunk0x2022
func (a *Ase) parseSlice(raw []byte, totalFrames int) Slice {
	var s Slice
	nKeysForSlice := int(binary.LittleEndian.Uint32(raw))
	flags := binary.LittleEndian.Uint32(raw[4:])
	name := parseString(raw[12:])
	raw = raw[14+len(name):]
	s.Name = name
	frameIndices := make([]int, 0, nKeysForSlice)
	for i := 0; len(raw) > 0 && i < nKeysForSlice; i++ {
		frameIdx := int(binary.LittleEndian.Uint32(raw))
		frameIndices = append(frameIndices, frameIdx)
		var key SliceFrame
		x := int32(binary.LittleEndian.Uint32(raw[4:]))
		y := int32(binary.LittleEndian.Uint32(raw[8:]))
		w := binary.LittleEndian.Uint32(raw[12:])
		h := binary.LittleEndian.Uint32(raw[16:])
		raw = raw[20:]
		key.Bounds = image.Rect(int(x), int(y), int(x)+int(w), int(y)+int(h))
		if flags&1 != 0 {
			cx := int32(binary.LittleEndian.Uint32(raw))
			cy := int32(binary.LittleEndian.Uint32(raw[4:]))
			cw := binary.LittleEndian.Uint32(raw[8:])
			ch := binary.LittleEndian.Uint32(raw[12:])
			raw = raw[16:]
			key.Rect9Slices = image.Rect(int(cx), int(cy), int(cx)+int(cw), int(cy)+int(ch))
		}
		if flags&2 != 0 {
			px := int32(binary.LittleEndian.Uint32(raw))
			py := int32(binary.LittleEndian.Uint32(raw[4:]))
			raw = raw[8:]
			key.Pivot = image.Pt(int(px), int(py))
		}
		s.Frames = append(s.Frames, key)
	}
	expandSliceKey(&s, totalFrames, frameIndices)
	return s
}

// Chunk0x2018
func (a *Ase) parseTags(raw []byte) []Tag {
	ntags := int(binary.LittleEndian.Uint16(raw))
	tags := make([]Tag, ntags)
	ptr := raw[10:]
	for i := range ntags {
		t := &tags[i]
		t.Start = binary.LittleEndian.Uint16(ptr)
		t.End = binary.LittleEndian.Uint16(ptr[2:])
		t.LoopDirection = LoopDirection(ptr[4])
		t.Repeat = binary.LittleEndian.Uint16(ptr[5:])
		nameLen := binary.LittleEndian.Uint16(ptr[17:])
		t.Name = string(ptr[19 : 19+nameLen])
		ptr = ptr[19+nameLen:]
	}

	return tags
}

// Chunk0x2020
func (a *Ase) parseUserData(raw []byte) (data []byte, col color.NRGBA) {
	flags := binary.LittleEndian.Uint32(raw)
	raw = raw[4:]
	if flags&1 != 0 {
		n := binary.LittleEndian.Uint16(raw)
		data, raw = raw[2:2+n], raw[2+n:]
	}
	if flags&2 != 0 {
		col = parseColor(raw)
	}
	return data, col
}

// Chunk0x2019
func (a *Ase) parsePalette(raw []byte) {
	entries := binary.LittleEndian.Uint32(raw[0:])
	lo := binary.LittleEndian.Uint32(raw[4:])
	raw = raw[20:]
	for i := 0; i < int(entries); i++ {
		if len(raw) < 2 {
			break
		}
		flags := binary.LittleEndian.Uint16(raw)
		if len(raw) < 6 {
			break
		}
		idx := int(lo) + i
		if idx < len(a.Palette) {
			a.Palette[idx] = parseColor(raw[2:])
		}
		raw = raw[6:]
		if flags&1 != 0 {
			raw = skipString(raw)
		}
	}
}

func (a *Ase) parseOldPalette0x0004(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for i := 0; i < int(packets); i++ {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256
		}
		raw = raw[2:]
		for j := 0; j < n && currentIndex < len(a.Palette); j++ {
			a.Palette[currentIndex] = color.NRGBA{
				R: raw[0],
				G: raw[1],
				B: raw[2],
				A: 255,
			}
			raw = raw[3:]
			currentIndex++
		}
	}
}

// Chunk0x0011
func (a *Ase) parseOldPalette0x0011(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for i := 0; i < int(packets); i++ {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256
		}
		raw = raw[2:]
		for j := 0; j < n && currentIndex < len(a.Palette); j++ {
			a.Palette[currentIndex] = color.NRGBA{
				R: raw[0] * 4,
				G: raw[1] * 4,
				B: raw[2] * 4,
				A: 255,
			}
			raw = raw[3:]
			currentIndex++
		}
	}
}

// Chunk0x2004
func (a *Ase) parseLayer(raw []byte) Layer {
	var l Layer

	flags := binary.LittleEndian.Uint16(raw)
	l.IsVisible = flags&1 != 0
	l.IsLocked = flags&2 != 0
	// l.IsLocked = flags&4 != 0
	l.IsBackground = flags&8 != 0
	l.PreferLinkedCels = flags&16 != 0
	l.IsCollapsed = flags&32 != 0
	l.IsReference = flags&64 != 0

	l.Type = LayerType(binary.LittleEndian.Uint16(raw[2:]))
	l.ChildLevel = binary.LittleEndian.Uint16(raw[4:])
	l.BlendMode = BlendMode(binary.LittleEndian.Uint16(raw[10:]))
	l.Opacity = raw[12]

	nameStartIndex := 16
	l.Name = parseString(raw[nameStartIndex:])

	if l.Type == Tilemap {
		// WORD + string data len
		nameLen := 2 + int(binary.LittleEndian.Uint16(raw[nameStartIndex:]))
		tilesetIndexOffset := nameStartIndex + nameLen
		l.TilesetIndex = binary.LittleEndian.Uint32(raw[tilesetIndexOffset:])
	}
	return l
}

// Chunk0x2005
func (a *Ase) parseCel(raw []byte, layerIdx int) (*Cel, error) {
	cel := Cel{}
	cel.LayerIndex = int(binary.LittleEndian.Uint16(raw[0:2]))

	if int(cel.LayerIndex) < len(a.Layers) {
		cel.Layer = a.Layers[cel.LayerIndex]
	}

	x := int(int16(binary.LittleEndian.Uint16(raw[2:])))
	y := int(int16(binary.LittleEndian.Uint16(raw[4:])))

	cel.Pos.X = x
	cel.Pos.Y = y
	cel.Opacity = raw[6]
	cel.Type = CelType(binary.LittleEndian.Uint16(raw[7:]))
	cel.ZIndex = int(int16(binary.LittleEndian.Uint16(raw[9:])))

	if layerIdx >= len(a.Layers) {
		return nil, nil
	}

	raw = raw[16:] // Cel Header bitti, veriye geçiyoruz

	var pix []byte
	cel.Size.X = int(binary.LittleEndian.Uint16(raw))
	cel.Size.Y = int(binary.LittleEndian.Uint16(raw[2:]))

	switch cel.Type {
	case RawUncompressed:
		pix = raw[4:]

	case LinkedCel:
		srcFrame := int(binary.LittleEndian.Uint16(raw))
		if srcFrame < len(a.Frames) && layerIdx < len(a.Frames[srcFrame].Cels) {
			c := a.Frames[srcFrame].Cels[layerIdx]
			return c, nil
		}
		return nil, nil

	case CompressedImage:
		zr, err := zlib.NewReader(bytes.NewReader(raw[4:]))
		if err != nil {
			return nil, err
		}
		defer zr.Close()

		data, err := io.ReadAll(zr)
		if err != nil {
			return nil, err
		}
		pix = data

	case CompressedTilemap:
		bitsPerTile := binary.LittleEndian.Uint16(raw[4:])
		tileIDMask := binary.LittleEndian.Uint32(raw[6:])
		xFlipMask := binary.LittleEndian.Uint32(raw[10:])
		yFlipMask := binary.LittleEndian.Uint32(raw[14:])
		dFlipMask := binary.LittleEndian.Uint32(raw[18:])

		raw = raw[32:]
		zr, _ := zlib.NewReader(bytes.NewReader(raw))
		tileBytes, _ := io.ReadAll(zr)

		bytesPerTile := int(bitsPerTile / 8)
		numTiles := len(tileBytes) / bytesPerTile
		cel.Tiles = make([]Tile, numTiles)

		for i := 0; i < numTiles; i++ {
			start := i * bytesPerTile
			var rawTile uint32

			if bytesPerTile == 4 {
				rawTile = binary.LittleEndian.Uint32(tileBytes[start : start+4])
			} else {
				rawTile = uint32(binary.LittleEndian.Uint16(tileBytes[start : start+2]))
			}

			var flags FlipBitMask
			if (rawTile & xFlipMask) != 0 {
				flags |= FlipX
			}
			if (rawTile & yFlipMask) != 0 {
				flags |= FlipY
			}
			if (rawTile & dFlipMask) != 0 {
				flags |= FlipD
			}

			cel.Tiles[i] = Tile{
				ID:  rawTile & tileIDMask,
				XYD: flags,
			}
		}
		return &cel, nil

	default:
		return nil, errors.New("unsupported cel type")
	}

	if len(pix) > 0 {
		bounds := image.Rect(0, 0, cel.Size.X, cel.Size.Y)
		var img image.Image

		switch a.ColorDepth {
		case Indexed:
			img = &image.Paletted{
				Pix:     pix,
				Stride:  cel.Size.X,
				Rect:    bounds,
				Palette: a.Palette,
			}
		case Grayscale:
			nrgba := image.NewNRGBA(bounds)
			stride := cel.Size.X * 2
			for dy := 0; dy < cel.Size.Y; dy++ {
				for dx := 0; dx < cel.Size.X; dx++ {
					i := dy*stride + dx*2
					if i+1 < len(pix) {
						g := pix[i]
						al := pix[i+1]
						nrgba.SetNRGBA(bounds.Min.X+dx, bounds.Min.Y+dy, color.NRGBA{
							R: g, G: g, B: g, A: al,
						})
					}
				}
			}
			img = nrgba
		case NRGBA:
			nrgba := &image.NRGBA{
				Pix:    pix,
				Stride: cel.Size.X * 4,
				Rect:   bounds,
			}
			img = nrgba
		default:
			return nil, errors.New("invalid color depth")
		}
		cel.Image = img
	}

	return &cel, nil
}
func (a *Ase) buildUserDataText() []byte {
	n := 0
	for _, l := range a.Layers {
		if l.IsVisible {
			n += len(l.Text)
		}
	}
	for _, fr := range a.Frames {
		for _, c := range fr.Cels {
			n += len(c.Text)
		}
	}
	return make([]byte, 0, n)
}
func (a *Ase) buildLayerUserDataText() [][]byte {
	userdataText := a.buildUserDataText()
	ld := make([][]byte, 0, len(a.Layers))
	for _, l := range a.Layers {
		if l.IsVisible && len(l.Text) > 0 {
			ofs := len(userdataText)
			userdataText = append(userdataText, l.Text...)
			ld = append(ld, userdataText[ofs:])
		}
	}
	return ld
}

type Frame struct {
	// Duration of this frame in the animation
	Duration time.Duration
	// Cels in this frame, ordered by layer index and group child level. that increase from bottom to top.
	Cels []*Cel
}

type SliceFrame struct {
	// The bounds of the slice in the canvas
	Bounds image.Rectangle
	// 9-slices internal rectangle (relative to slice bounds)
	Rect9Slices image.Rectangle
	// A pivot to specify the central/base location. (relative to slice bounds)
	Pivot image.Point
}

type Slice struct {
	UserData
	Name   string
	Frames []SliceFrame
}

// A cel is an image in a specific xy-coordinate, and a specific layer/frame combination.
type Cel struct {
	// LayerIndex is the raw index of the layer this cel belongs to.
	LayerIndex int

	// ZIndex defines the display order override.
	// 0 = default, +N = show N layers forward, -N = show N layers back.
	ZIndex int

	// Cel position
	Pos image.Point

	// Cel Width and Cel Height. Pixel unit for Image layers.
	//
	// Number of tiles for Tilemap layers.
	Size image.Point

	// Cel image
	Image image.Image

	// Cel opacity. (0-255)
	Opacity uint8

	// Tiles contains the ID and flip flags for each tile in a tilemap layer.
	// This is nil for regular image layers.
	Tiles []Tile

	// UserData contains extra information like text and color associated with the cel.
	UserData

	// Type specifies if this cel is a Raw Image, Linked Cel, Compressed Image, or Tilemap.
	Type CelType

	// Layer is a reference to the layer this cel belongs to,
	Layer *Layer
}

type Tile struct {
	ID  uint32      // Tile index
	XYD FlipBitMask // XYD bitmask flags
}

// IsEmpty checks if the Cel is empty (has no image).
func (c *Cel) IsEmpty() bool {
	return c.Image == nil
}

func (c *Cel) setUserData(text string, col color.NRGBA) {
	c.Text = text
	c.Color = col
}

type LayerFlags struct {
	IsVisible        bool
	IsLocked         bool
	IsBackground     bool
	PreferLinkedCels bool
	IsCollapsed      bool
	IsReference      bool
}

// Layer data
type Layer struct {
	UserData
	Type LayerType
	// Layer name.
	Name string
	// Blending mode of this layer.
	BlendMode BlendMode
	// Layer opacity. (0-255)
	Opacity uint8

	LayerFlags
	ChildLevel uint16

	// Index for Ase.Tilesets[]
	TilesetIndex uint32
	Tileset      *Tileset
}

func (l *Layer) setUserData(text string, col color.NRGBA) {
	l.Text = text
	l.Color = col
}

// User-defined data
type UserData struct {
	Color color.NRGBA
	Text  string
}

func (u *UserData) setUserData(text string, c color.NRGBA) {
	u.Text = text
	u.Color = c
}

// Tag Represents a tag in the timeline.
type Tag struct {
	UserData
	Name string
	// Frame index where this tag starts.
	Start uint16
	// Frame index where this tag ends.
	End uint16
	// Play this animation section N times, 0 means infinite.
	Repeat        uint16
	LoopDirection LoopDirection
}

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
func factorPowerOfTwo(n int) (a, b int) {
	x := int(math.Ceil(math.Log2(float64(n))))
	a = 1 << (x - x/2)
	b = 1 << (x / 2)
	return
}

func Read(filepath string, onlyVisibleLayers bool) (a Ase, err error) {
	file, err := os.Open(filepath)
	if err != nil {
		return a, err
	}
	defer file.Close()

	_, err = a.parse(file, onlyVisibleLayers)

	if err != nil {
		return a, err
	}

	return a, nil
}

func ReadFs(f fs.FS, filepath string, onlyVisibleLayers bool) (a Ase, err error) {
	file, err := f.Open(filepath)
	if err != nil {
		return a, err
	}
	defer file.Close()

	_, err = a.parse(file, onlyVisibleLayers)

	if err != nil {
		return a, err
	}

	return a, nil
}
