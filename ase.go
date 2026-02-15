// Aseprite file parser/decoder
package aseprite

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"io"
	"slices"
	"time"

	"github.com/google/uuid"
)

type Ase struct {
	ColorDepth ColorDepth

	// Canvas size (width and height)
	Size image.Point

	// Pixel aspect ratio (width:height)
	PixelRatio image.Point

	// Timeline frame durations
	Durations []time.Duration

	// Layers. Layer indices increase from bottom to top in Aseprite UI.
	Layers []*Layer

	// HasLayersUUID indicates whether each layer in the file contains a Universally Unique Identifier.
	HasLayersUUID bool

	// Aseprite slices. https://www.aseprite.org/api/slice#slice
	Slices []Slice

	// Timeline animation tags
	Tags []Tag

	// Tilesets used in Tilemap layers
	Tilesets []*Tileset

	// Palette of file
	Palette color.Palette

	// Palette index for used as transparent color in each layer. (only for indexed images)
	TransparentIndex uint8

	ColorProfile ColorProfile
}

// GetLayerByName returns the first layer with the given name.
// Returns nil if no layer is found with that name.
func (a *Ase) GetLayerByName(name string) *Layer {
	idx := slices.IndexFunc(a.Layers, func(l *Layer) bool {
		return l.Name == name
	})
	if idx == -1 {
		return nil
	}
	return a.Layers[idx]
}

// GetLayerByUUID returns the layer with the given UUID.
// Returns nil if no layer is found with that UUID.
func (a *Ase) GetLayerByUUID(id uuid.UUID) *Layer {
	idx := slices.IndexFunc(a.Layers, func(l *Layer) bool {
		return l.UUID == id
	})
	if idx == -1 {
		return nil
	}
	return a.Layers[idx]
}

// BuildTilemapImages performs the final rasterization for all cels in tilemap layers.
// It iterates through the tile grid, resolves tile IDs via the associated tileset,
// and assembles the final image composite by applying bitmask flips (X/Y/D).
func (a *Ase) BuildTilemapImages() {
	for _, lyr := range a.Layers {
		if lyr.Type == Tilemap {
			for _, cel := range lyr.Cels {
				if cel != nil && cel.Type == CompressedTilemap {
					cel.BuildTilemapImage()
				}
			}
		}
	}
}

func (a *Ase) parse(r io.Reader, onlyVisible bool) (filesize int64, err error) {
	var hdr [128]byte
	if n, err := io.ReadFull(r, hdr[:]); err != nil {
		return int64(n), err
	}
	if magic := binary.LittleEndian.Uint16(hdr[4:]); magic != 0xA5E0 {
		return 128, errors.New("invalid magic number")
	}

	// pixel ratio
	a.PixelRatio.X = int(hdr[34])
	a.PixelRatio.Y = int(hdr[35])
	if a.PixelRatio.X == 0 || a.PixelRatio.Y == 0 {
		a.PixelRatio.X = 1
		a.PixelRatio.Y = 1
	}

	fileSize := int64(binary.LittleEndian.Uint32(hdr[:]))
	totalFrames := int(binary.LittleEndian.Uint16(hdr[6:]))
	a.Size.X = int(binary.LittleEndian.Uint16(hdr[8:]))
	a.Size.Y = int(binary.LittleEndian.Uint16(hdr[10:]))
	a.ColorDepth = ColorDepth(binary.LittleEndian.Uint16(hdr[12:]))

	// Flags (offset 14) - Bit 4: Layers have an UUID
	uuidflags := binary.LittleEndian.Uint32(hdr[14:])
	a.HasLayersUUID = (uuidflags & 4) != 0

	a.TransparentIndex = hdr[28]
	paletteSize := binary.LittleEndian.Uint16(hdr[32:])

	a.Palette = make(color.Palette, paletteSize)
	for i := range a.Palette {
		a.Palette[i] = color.Black
	}
	a.Palette[a.TransparentIndex] = color.Transparent

	a.Durations = make([]time.Duration, 0, totalFrames)

	chunkHeaderBuf := make([]byte, 6)
	frameHeaderBuf := make([]byte, 16)
	currentOffset := int64(128)

	for i := range totalFrames {
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

		currentDuration := time.Millisecond * time.Duration(durationMS)

		var lastUserDataTarget userDataReceiver
		var tagQueue []userDataReceiver

		for range nchunks {
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
			chunkData := make([]byte, dataLen)
			if _, err := io.ReadFull(r, chunkData); err != nil {
				return currentOffset, err
			}
			currentOffset += int64(dataLen)

			switch chunkType {
			case 0x2004: // Layer Chunk
				l := a.parseLayer(chunkData)
				l.parent = a
				l.rawIndex = len(a.Layers)
				// Pre-allocate Cels slice for the layer to match total frames
				l.Cels = make([]*Cel, totalFrames)
				a.Layers = append(a.Layers, l)
				lastUserDataTarget = a.Layers[len(a.Layers)-1]

			case 0x2005: // Cel Chunk
				layerIdx := int(binary.LittleEndian.Uint16(chunkData))
				cel, err := a.parseCel(chunkData, layerIdx)
				if err != nil {
					return currentOffset, err
				}
				if cel != nil {
					// Store cel directly in the layer's frame index
					if layerIdx < len(a.Layers) {
						cel.layer = a.Layers[layerIdx]
						a.Layers[layerIdx].Cels[i] = cel
						lastUserDataTarget = cel
					}
				}
			case 0x2007: // Color Profile Chunk
				cp, err := a.parseColorProfile(chunkData)
				if err != nil {
					return 0, err
				}
				a.ColorProfile = cp

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
				for k := range newTags {
					tagQueue = append(tagQueue, &a.Tags[startIdx+k])
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
		a.Durations = append(a.Durations, currentDuration)
	}

	// Filter layers based on visibility
	keptLayers := make([]*Layer, 0, len(a.Layers))
	levelVisibility := make(map[uint16]bool)

	for _, l := range a.Layers {
		parentVisible := true
		if l.ChildLevel > 0 {
			if visible, ok := levelVisibility[l.ChildLevel-1]; ok {
				parentVisible = visible
			}
		}

		isEffectiveVisible := l.IsVisible() && parentVisible
		levelVisibility[l.ChildLevel] = isEffectiveVisible

		if onlyVisible && !isEffectiveVisible {
			continue
		}
		keptLayers = append(keptLayers, l)
	}
	a.Layers = keptLayers

	// Link tilesets to layers if they are tilemaps
	for _, lyr := range a.Layers {
		if lyr.Type == Tilemap && int(lyr.TilesetIndex) < len(a.Tilesets) {
			lyr.tileset = a.Tilesets[lyr.TilesetIndex]
		}
	}

	return fileSize, nil
}

// Chunk0x2004
func (a *Ase) parseLayer(raw []byte) *Layer {
	var l Layer

	l.Flags = binary.LittleEndian.Uint16(raw)
	l.Type = LayerType(binary.LittleEndian.Uint16(raw[2:]))
	l.ChildLevel = binary.LittleEndian.Uint16(raw[4:])
	l.BlendMode = BlendMode(binary.LittleEndian.Uint16(raw[10:]))
	l.Opacity = raw[12]

	nameStartIndex := 16
	nameLen := int(binary.LittleEndian.Uint16(raw[nameStartIndex:]))
	l.Name = parseString(raw[nameStartIndex:])

	currentOffset := nameStartIndex + 2 + nameLen

	if l.Type == Tilemap {
		l.TilesetIndex = binary.LittleEndian.Uint32(raw[currentOffset:])
		currentOffset += 4
	}

	if a.HasLayersUUID {
		// Dilimi [16]byte tipine çevirip doğrudan atıyoruz
		l.UUID = uuid.UUID(raw[currentOffset : currentOffset+16])
	} else {
		l.UUID = uuid.Nil
	}

	return &l
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
	for range nKeysForSlice {
		if len(raw) == 0 {
			break
		}
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
		t.Start = int(binary.LittleEndian.Uint16(ptr))
		t.End = int(binary.LittleEndian.Uint16(ptr[2:]))
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

func (a *Ase) parseColorProfile(data []byte) (ColorProfile, error) {
	cp := ColorProfile{}
	if len(data) < 16 {
		return cp, errors.New("chunk data too short for header")
	}

	cp.Type = binary.LittleEndian.Uint16(data[0:2])
	cp.Flags = binary.LittleEndian.Uint16(data[2:4])
	fixedGamma := binary.LittleEndian.Uint32(data[4:8])
	cp.Gamma = float64(fixedGamma) / 65536.0

	// data[8:16] arası rezerve alan (atlandı)

	if cp.Type == 2 {
		if len(data) < 20 {
			return cp, errors.New("chunk data too short for icc length") // burası hala test edilmedi
		}

		iccLen := binary.LittleEndian.Uint32(data[16:20])

		if uint32(len(data)) < 20+iccLen {
			return cp, errors.New("icc profile data truncated") // burası hala test edilmedi
		}

		cp.ICC = make([]byte, iccLen)
		copy(cp.ICC, data[20:20+iccLen])
	}

	return cp, nil
}

// Chunk0x2019
func (a *Ase) parsePalette(raw []byte) {
	entries := binary.LittleEndian.Uint32(raw[0:])
	lo := binary.LittleEndian.Uint32(raw[4:])
	raw = raw[20:]
	for i := range int(entries) {
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
			for y := range tilesetBounds.Dy() {
				for x := range tilesetBounds.Dx() {
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

// Chunk0x2005
func (a *Ase) parseCel(raw []byte, layerIdx int) (*Cel, error) {
	if len(raw) < 16 {
		return nil, errors.New("cel chunk too short")
	}

	cel := Cel{}
	cel.LayerIndex = int(binary.LittleEndian.Uint16(raw[0:2]))

	if int(cel.LayerIndex) < len(a.Layers) {
		cel.layer = a.Layers[cel.LayerIndex]
	}

	cel.Pos.X = int(int16(binary.LittleEndian.Uint16(raw[2:4])))
	cel.Pos.Y = int(int16(binary.LittleEndian.Uint16(raw[4:6])))
	cel.Opacity = raw[6]

	cel.Type = CelType(binary.LittleEndian.Uint16(raw[7:9]))
	cel.ZIndex = int(int16(binary.LittleEndian.Uint16(raw[9:11])))

	if layerIdx >= len(a.Layers) {
		return nil, nil
	}

	// Move pointer past the 16-byte header
	raw = raw[16:]

	var pix []byte

	switch cel.Type {
	case LinkedCel:
		if len(raw) < 2 {
			return nil, errors.New("linked cel data too short")
		}
		srcFrame := int(binary.LittleEndian.Uint16(raw))

		// Access the source cel from the same layer but at the referenced frame
		lyr := a.Layers[layerIdx]
		if srcFrame < len(lyr.Cels) {
			return lyr.Cels[srcFrame], nil
		}
		return nil, nil

	case RawUncompressed:
		if len(raw) < 4 {
			return nil, errors.New("raw cel size data missing")
		}
		cel.Size.X = int(binary.LittleEndian.Uint16(raw[0:2]))
		cel.Size.Y = int(binary.LittleEndian.Uint16(raw[2:4]))
		pix = raw[4:]

	case CompressedImage:
		if len(raw) < 4 {
			return nil, errors.New("compressed cel size data missing")
		}
		cel.Size.X = int(binary.LittleEndian.Uint16(raw[0:2]))
		cel.Size.Y = int(binary.LittleEndian.Uint16(raw[2:4]))

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
		if len(raw) < 4 {
			return nil, errors.New("tilemap cel size data missing")
		}
		cel.Size.X = int(binary.LittleEndian.Uint16(raw[0:2]))
		cel.Size.Y = int(binary.LittleEndian.Uint16(raw[2:4]))

		bitsPerTile := binary.LittleEndian.Uint16(raw[4:6])
		tileIDMask := binary.LittleEndian.Uint32(raw[6:10])
		xFlipMask := binary.LittleEndian.Uint32(raw[10:14])
		yFlipMask := binary.LittleEndian.Uint32(raw[14:18])
		dFlipMask := binary.LittleEndian.Uint32(raw[18:22])

		// Data starts at offset 32
		zr, err := zlib.NewReader(bytes.NewReader(raw[32:]))
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		tileBytes, _ := io.ReadAll(zr)

		bytesPerTile := int(bitsPerTile / 8)
		numTiles := len(tileBytes) / bytesPerTile
		cel.Tiles = make([]Tile, numTiles)

		for i := range numTiles {
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

	// Process pixel data for non-tilemap types
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
			for dy := range cel.Size.Y {
				for dx := range cel.Size.X {
					i := dy*stride + dx*2
					if i+1 < len(pix) {
						g := pix[i]
						al := pix[i+1]
						nrgba.SetNRGBA(dx, dy, color.NRGBA{R: g, G: g, B: g, A: al})
					}
				}
			}
			img = nrgba
		case NRGBA:
			img = &image.NRGBA{
				Pix:    pix,
				Stride: cel.Size.X * 4,
				Rect:   bounds,
			}
		default:
			return nil, errors.New("invalid color depth")
		}
		cel.Image = img
	}

	return &cel, nil
}

func (a *Ase) parseOldPalette0x0004(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for range int(packets) {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256
		}
		raw = raw[2:]
		for range n {
			if currentIndex >= len(a.Palette) {
				break
			}
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

func (a *Ase) parseOldPalette0x0011(raw []byte) {
	packets := binary.LittleEndian.Uint16(raw)
	raw = raw[2:]
	currentIndex := 0
	for range int(packets) {
		skip := int(raw[0])
		currentIndex += skip
		n := int(raw[1])
		if n == 0 {
			n = 256 // bu kısım test yüzdesini düşürüyor
		}
		raw = raw[2:]
		for range n {
			if currentIndex >= len(a.Palette) {
				break
			}
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
